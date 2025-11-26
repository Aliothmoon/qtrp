package transport

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

// QUICConfig QUIC 传输配置
type QUICConfig struct {
	// TLS 配置（服务端必需）
	TLSConfig *tls.Config

	// 客户端跳过证书验证
	InsecureSkipVerify bool

	// 连接超时
	DialTimeout time.Duration
}

// QUICTransport QUIC 传输层实现
type QUICTransport struct {
	config *QUICConfig
}

// NewQUICTransport 创建 QUIC 传输层（用于测试）
func NewQUICTransport() *QUICTransport {
	return &QUICTransport{config: &QUICConfig{
		InsecureSkipVerify: true,
		DialTimeout:        10 * time.Second,
	}}
}

// NewQUICTransportWithConfig 创建带配置的 QUIC 传输层
func NewQUICTransportWithConfig(cfg *QUICConfig) *QUICTransport {
	if cfg == nil {
		cfg = &QUICConfig{}
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 10 * time.Second
	}
	return &QUICTransport{config: cfg}
}

func (t *QUICTransport) Type() string {
	return "quic"
}

// Listen 监听 QUIC 连接
func (t *QUICTransport) Listen(addr string) (Listener, error) {
	tlsConfig := t.config.TLSConfig
	if tlsConfig == nil {
		// 没有提供 TLS 配置时，生成临时证书（仅用于测试）
		tlsConfig = generateTempTLSConfig()
	}

	listener, err := quic.ListenAddr(addr, tlsConfig, generateQUICConfig())
	if err != nil {
		return nil, err
	}

	return &QUICListener{listener: listener}, nil
}

// Dial 连接到 QUIC 服务器
func (t *QUICTransport) Dial(addr string) (Conn, error) {
	tlsConfig := t.config.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: t.config.InsecureSkipVerify,
			NextProtos:         []string{"qtrp"},
			MinVersion:         tls.VersionTLS13,
		}
	} else if t.config.InsecureSkipVerify {
		// 客户端模式下强制设置 InsecureSkipVerify
		tlsConfig = tlsConfig.Clone()
		tlsConfig.InsecureSkipVerify = true
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.config.DialTimeout)
	defer cancel()

	conn, err := quic.DialAddr(ctx, addr, tlsConfig, generateQUICConfig())
	if err != nil {
		return nil, err
	}

	return &QUICConn{conn: conn, isServer: false}, nil
}

// QUICListener QUIC 监听器
type QUICListener struct {
	listener *quic.Listener
}

func (l *QUICListener) Accept() (Conn, error) {
	ctx := context.Background()
	conn, err := l.listener.Accept(ctx)
	if err != nil {
		return nil, err
	}
	return &QUICConn{conn: conn, isServer: true}, nil
}

func (l *QUICListener) Close() error {
	return l.listener.Close()
}

func (l *QUICListener) Addr() net.Addr {
	return l.listener.Addr()
}

// QUICConn QUIC 连接（单流模式，用于兼容现有接口）
type QUICConn struct {
	conn       *quic.Conn
	stream     *quic.Stream
	streamInit bool
	isServer   bool // 标识是服务端还是客户端连接
	ctx        interface{}
	mu         sync.Mutex // 保护流初始化
}

func (c *QUICConn) SetContext(ctx interface{}) {
	c.ctx = ctx
}

func (c *QUICConn) Context() interface{} {
	return c.ctx
}

// initStream 初始化流（服务端等待流，客户端打开流）
func (c *QUICConn) initStream() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.streamInit {
		return nil
	}

	var stream *quic.Stream
	var err error

	if c.isServer {
		// 服务端：等待客户端打开流
		stream, err = c.conn.AcceptStream(context.Background())
	} else {
		// 客户端：主动打开流
		stream, err = c.conn.OpenStreamSync(context.Background())
	}

	if err != nil {
		return err
	}
	c.stream = stream
	c.streamInit = true
	return nil
}

func (c *QUICConn) Read(b []byte) (int, error) {
	if err := c.initStream(); err != nil {
		return 0, err
	}
	return c.stream.Read(b)
}

func (c *QUICConn) Write(b []byte) (int, error) {
	if err := c.initStream(); err != nil {
		return 0, err
	}
	return c.stream.Write(b)
}

func (c *QUICConn) Close() error {
	if c.streamInit {
		c.stream.Close()
	}
	return c.conn.CloseWithError(0, "closed")
}

func (c *QUICConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *QUICConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *QUICConn) SetDeadline(t time.Time) error {
	if c.streamInit {
		return c.stream.SetDeadline(t)
	}
	return nil
}

func (c *QUICConn) SetReadDeadline(t time.Time) error {
	if c.streamInit {
		return c.stream.SetReadDeadline(t)
	}
	return nil
}

func (c *QUICConn) SetWriteDeadline(t time.Time) error {
	if c.streamInit {
		return c.stream.SetWriteDeadline(t)
	}
	return nil
}

// generateQUICConfig 生成优化的 QUIC 配置
func generateQUICConfig() *quic.Config {
	return &quic.Config{
		MaxIdleTimeout:                 30 * time.Second,
		MaxIncomingStreams:             1000,
		MaxIncomingUniStreams:          1000,
		InitialStreamReceiveWindow:     2 * 1024 * 1024,  // 2MB
		MaxStreamReceiveWindow:         16 * 1024 * 1024, // 16MB
		InitialConnectionReceiveWindow: 4 * 1024 * 1024,  // 4MB
		MaxConnectionReceiveWindow:     32 * 1024 * 1024, // 32MB
		KeepAlivePeriod:                15 * time.Second,
		EnableDatagrams:                true,
	}
}

// generateTempTLSConfig 生成临时 TLS 配置（仅用于测试）
func generateTempTLSConfig() *tls.Config {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, _ := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	privBytes, _ := x509.MarshalECPrivateKey(priv)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	cert, _ := tls.X509KeyPair(certPEM, keyPEM)
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"qtrp"},
		MinVersion:   tls.VersionTLS13,
	}
}
