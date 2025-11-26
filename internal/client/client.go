package client

import (
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"qtrp/pkg/auth"
	"qtrp/pkg/config"
	"qtrp/pkg/crypto"
	"qtrp/pkg/mux"
	"qtrp/pkg/pool"
	"qtrp/pkg/protocol"
	"qtrp/pkg/transport"
)

// Client 客户端
type Client struct {
	config  *config.ClientConfig
	trans   transport.Transport
	crypt   crypto.Crypto
	session *mux.Session

	ctrlStream net.Conn
	proxies    map[string]*config.ExpandedProxy // name -> ExpandedProxy

	closeCh   chan struct{}
	reconnect bool
	mu        sync.RWMutex
	wg        sync.WaitGroup
}

// New 创建客户端
func New(cfg *config.ClientConfig) (*Client, error) {
	// 展开所有代理配置（端口范围 -> 多个单端口代理）
	proxies := make(map[string]*config.ExpandedProxy)
	for _, proxyCfg := range cfg.Proxies {
		expanded, err := proxyCfg.Expand()
		if err != nil {
			return nil, err
		}
		for _, ep := range expanded {
			proxies[ep.Name] = ep
		}
	}

	var trans transport.Transport
	var crypt crypto.Crypto

	// QUIC 传输层配置
	if cfg.Transport == "quic" {
		trans = transport.NewWithQUICConfig("quic", &transport.QUICConfig{
			InsecureSkipVerify: cfg.QUIC.SkipVerify,
		})
		// QUIC 自带加密，不需要额外加密层
		crypt = crypto.New("none", nil)
		log.Printf("[client] using QUIC transport (skip_verify=%v)", cfg.QUIC.SkipVerify)
	} else {
		trans = transport.New(cfg.Transport)
		crypt = crypto.New(cfg.Crypto, &crypto.Config{
			CACert:     cfg.TLS.CACert,
			SkipVerify: cfg.TLS.SkipVerify,
			PSK:        cfg.ChaCha20.PSK,
		})
	}

	return &Client{
		config:    cfg,
		trans:     trans,
		crypt:     crypt,
		proxies:   proxies,
		closeCh:   make(chan struct{}),
		reconnect: true,
	}, nil
}

// Run 运行客户端
func (c *Client) Run() error {
	for {
		if err := c.connect(); err != nil {
			log.Printf("[client] connect error: %v", err)
		}

		select {
		case <-c.closeCh:
			return nil
		default:
		}

		if !c.reconnect {
			return nil
		}

		log.Printf("[client] reconnecting in 5 seconds...")
		time.Sleep(5 * time.Second)
	}
}

// Stop 停止客户端
func (c *Client) Stop() {
	c.mu.Lock()
	c.reconnect = false
	close(c.closeCh)
	c.mu.Unlock()

	if c.session != nil {
		c.session.Close()
	}
	c.wg.Wait()
}

func (c *Client) connect() error {
	log.Printf("[client] connecting to %s (transport=%s, crypto=%s)",
		c.config.ServerAddr, c.trans.Type(), c.crypt.Type())

	// 建立连接
	conn, err := c.trans.Dial(c.config.ServerAddr)
	if err != nil {
		log.Printf("[client] dial server error: %v", err)
		return err
	}

	// 优化 TCP 参数
	if err := pool.OptimizeTCPConn(conn); err != nil {
		log.Printf("[client] optimize TCP params error: %v", err)
		// 继续执行，优化失败不影响功能
	}

	// 加密包装
	cryptConn, err := c.crypt.WrapConn(conn, false)
	if err != nil {
		log.Printf("[client] crypto wrap error: %v", err)
		conn.Close()
		return err
	}

	// 创建多路复用会话
	session, err := mux.NewClientSession(cryptConn)
	if err != nil {
		log.Printf("[client] create mux session error: %v", err)
		cryptConn.Close()
		return err
	}
	c.session = session

	// 打开控制流
	ctrlStream, err := session.OpenStream()
	if err != nil {
		log.Printf("[client] open control stream error: %v", err)
		session.Close()
		return err
	}
	c.ctrlStream = ctrlStream

	// 认证
	if err := c.authenticate(); err != nil {
		log.Printf("[client] authentication failed: %v", err)
		ctrlStream.Close()
		session.Close()
		return err
	}

	log.Printf("[client] authenticated successfully")

	// 注册代理（已展开的）
	successCount := 0
	for _, proxy := range c.proxies {
		if err := c.registerProxy(proxy); err != nil {
			log.Printf("[client] register proxy %s error: %v", proxy.Name, err)
		} else {
			successCount++
		}
	}

	if successCount == 0 && len(c.proxies) > 0 {
		log.Printf("[client] warning: no proxies registered successfully")
	} else {
		log.Printf("[client] %d/%d proxies registered successfully", successCount, len(c.proxies))
	}

	// 启动心跳
	c.wg.Add(1)
	go c.keepalive()

	// 处理控制消息
	c.handleControlStream()

	return nil
}

func (c *Client) authenticate() error {
	ts := time.Now().Unix()
	nonce := auth.GenerateNonce()
	sign := auth.Sign(c.config.Token, strconv.FormatInt(ts, 10), nonce)

	req := protocol.AuthRequest{
		Token:     c.config.Token,
		Timestamp: ts,
		Nonce:     nonce,
		Sign:      sign,
		Version:   "1.0.0",
	}

	if err := protocol.SendMessage(c.ctrlStream, protocol.MsgTypeAuth, req); err != nil {
		log.Printf("[client] send auth request error: %v", err)
		return err
	}

	msg, err := protocol.RecvMessage(c.ctrlStream)
	if err != nil {
		log.Printf("[client] recv auth response error: %v", err)
		return err
	}

	if msg.Type != protocol.MsgTypeAuthResp {
		log.Printf("[client] expected auth response, got type %d", msg.Type)
		return &AuthError{Message: "invalid response type"}
	}

	var resp protocol.AuthResponse
	if err := msg.Decode(&resp); err != nil {
		log.Printf("[client] decode auth response error: %v", err)
		return err
	}

	if !resp.OK {
		log.Printf("[client] authentication rejected: %s", resp.Message)
		return &AuthError{Message: resp.Message}
	}

	return nil
}

func (c *Client) registerProxy(proxy *config.ExpandedProxy) error {
	req := protocol.NewProxyRequest{
		Name:       proxy.Name,
		Type:       proxy.Type,
		RemotePort: proxy.RemotePort,
	}

	if err := protocol.SendMessage(c.ctrlStream, protocol.MsgTypeNewProxy, req); err != nil {
		log.Printf("[client] send NewProxy request for %s error: %v", proxy.Name, err)
		return err
	}

	msg, err := protocol.RecvMessage(c.ctrlStream)
	if err != nil {
		log.Printf("[client] recv NewProxy response for %s error: %v", proxy.Name, err)
		return err
	}

	if msg.Type != protocol.MsgTypeNewProxyResp {
		log.Printf("[client] expected NewProxyResp, got type %d", msg.Type)
		return &ProxyError{Name: proxy.Name, Message: "invalid response type"}
	}

	var resp protocol.NewProxyResponse
	if err := msg.Decode(&resp); err != nil {
		log.Printf("[client] decode NewProxy response for %s error: %v", proxy.Name, err)
		return err
	}

	if !resp.OK {
		log.Printf("[client] proxy %s registration failed: %s", proxy.Name, resp.Message)
		return &ProxyError{Name: proxy.Name, Message: resp.Message}
	}

	log.Printf("[client] proxy %s registered: remote port %d -> local %s",
		proxy.Name, resp.RemotePort, proxy.GetLocalAddress())

	return nil
}

func (c *Client) handleControlStream() {
	for {
		msg, err := protocol.RecvMessage(c.ctrlStream)
		if err != nil {
			if err != io.EOF {
				log.Printf("[client] recv control message error: %v", err)
			}
			return
		}

		switch msg.Type {
		case protocol.MsgTypeNewConn:
			var req protocol.NewConnRequest
			if err := msg.Decode(&req); err != nil {
				log.Printf("[client] decode NewConn request error: %v", err)
				continue
			}
			c.wg.Add(1)
			go c.handleNewConn(req)

		case protocol.MsgTypePong:
			// 心跳响应
		default:
			log.Printf("[client] unknown message type: %d", msg.Type)
		}
	}
}

func (c *Client) handleNewConn(req protocol.NewConnRequest) {
	defer c.wg.Done()

	proxy, exists := c.proxies[req.ProxyName]
	if !exists {
		log.Printf("[client] unknown proxy: %s", req.ProxyName)
		return
	}

	localAddr := proxy.GetLocalAddress()

	// 连接本地服务
	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		log.Printf("[client] proxy %s connect to local %s error: %v", req.ProxyName, localAddr, err)
		return
	}
	defer localConn.Close()

	// 优化本地连接 TCP 参数
	if err := pool.OptimizeTCPConn(localConn); err != nil {
		log.Printf("[client] proxy %s optimize local conn TCP params error: %v", req.ProxyName, err)
		// 继续执行，优化失败不影响功能
	}

	// 打开数据流
	dataStream, err := c.session.OpenStream()
	if err != nil {
		log.Printf("[client] proxy %s open data stream error: %v", req.ProxyName, err)
		return
	}
	defer dataStream.Close()

	// 双向转发
	Join(localConn, dataStream)
}

func (c *Client) keepalive() {
	defer c.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := protocol.SendMessage(c.ctrlStream, protocol.MsgTypePing, protocol.Ping{
				Timestamp: time.Now().UnixMilli(),
			}); err != nil {
				log.Printf("[client] send ping error: %v", err)
				return
			}
		case <-c.closeCh:
			log.Printf("[client] keepalive stopped")
			return
		}
	}
}

// AuthError 认证错误
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return "authentication failed: " + e.Message
}

// ProxyError 代理错误
type ProxyError struct {
	Name    string
	Message string
}

func (e *ProxyError) Error() string {
	return "proxy " + e.Name + " failed: " + e.Message
}
