package crypto

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
)

// TLSCrypto TLS 加密实现
type TLSCrypto struct {
	config *Config
}

// NewTLSCrypto 创建 TLS 加密层
func NewTLSCrypto(cfg *Config) *TLSCrypto {
	return &TLSCrypto{config: cfg}
}

func (t *TLSCrypto) Type() string {
	return "tls"
}

// WrapConn 包装连接为 TLS 连接
func (t *TLSCrypto) WrapConn(conn net.Conn, isServer bool) (net.Conn, error) {
	if isServer {
		return t.wrapServer(conn)
	}
	return t.wrapClient(conn)
}

func (t *TLSCrypto) wrapServer(conn net.Conn) (net.Conn, error) {
	cert, err := tls.LoadX509KeyPair(t.config.CertFile, t.config.KeyFile)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	tlsConn := tls.Server(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func (t *TLSCrypto) wrapClient(conn net.Conn) (net.Conn, error) {
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: t.config.SkipVerify,
	}

	// 加载 CA 证书
	if t.config.CACert != "" {
		caCert, err := os.ReadFile(t.config.CACert)
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}
	return tlsConn, nil
}

// WrapListener 包装监听器为 TLS 监听器
func (t *TLSCrypto) WrapListener(ln net.Listener) (net.Listener, error) {
	cert, err := tls.LoadX509KeyPair(t.config.CertFile, t.config.KeyFile)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	return tls.NewListener(ln, tlsConfig), nil
}
