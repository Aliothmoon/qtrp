package server

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
	"qtrp/pkg/stats"
	"qtrp/pkg/transport"
)

// Server 服务端
type Server struct {
	config   *config.ServerConfig
	trans    transport.Transport
	crypt    crypto.Crypto
	auth     *auth.Authenticator
	listener transport.Listener

	clients sync.Map // clientID -> *Client
	proxies sync.Map // remotePort -> *Proxy

	closeCh chan struct{}
	wg      sync.WaitGroup
}

// Client 客户端连接
type Client struct {
	id       string
	session  *mux.Session
	proxies  map[string]*Proxy
	lastPing time.Time
	mu       sync.RWMutex
}

// Proxy 代理
type Proxy struct {
	name       string
	remotePort int
	listener   net.Listener
	client     *Client
	closeCh    chan struct{}
}

// New 创建服务端
func New(cfg *config.ServerConfig) *Server {
	var trans transport.Transport

	// QUIC 传输层需要在启动时生成证书
	if cfg.Transport == "quic" {
		certMgr, err := crypto.NewCertManager(&crypto.CertConfig{
			CertFile:     cfg.QUIC.CertFile,
			KeyFile:      cfg.QUIC.KeyFile,
			Organization: "QTRP",
			CommonName:   "qtrp-server",
		})
		if err != nil {
			log.Fatalf("[server] create certificate manager error: %v", err)
		}
		log.Printf("[server] QUIC certificate initialized")

		trans = transport.NewWithQUICConfig("quic", &transport.QUICConfig{
			TLSConfig: certMgr.GetServerTLSConfig(),
		})
	} else {
		trans = transport.New(cfg.Transport)
	}

	// QUIC 自带加密，不需要额外加密层
	var crypt crypto.Crypto
	if cfg.Transport == "quic" {
		crypt = crypto.New("none", nil)
	} else {
		crypt = crypto.New(cfg.Crypto, &crypto.Config{
			CertFile: cfg.TLS.CertFile,
			KeyFile:  cfg.TLS.KeyFile,
			PSK:      cfg.ChaCha20.PSK,
		})
	}

	return &Server{
		config:  cfg,
		trans:   trans,
		crypt:   crypt,
		auth:    auth.NewAuthenticator(cfg.Auth.Tokens),
		closeCh: make(chan struct{}),
	}
}

// Start 启动服务端
func (s *Server) Start() error {
	ln, err := s.trans.Listen(s.config.BindAddr)
	if err != nil {
		return err
	}
	s.listener = ln
	log.Printf("[server] listening on %s (transport=%s, crypto=%s)",
		s.config.BindAddr, s.trans.Type(), s.crypt.Type())

	// 启动 Dashboard
	if s.config.DashboardAddr != "" {
		go s.startDashboard(s.config.DashboardAddr)
	}

	// 接受连接
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.closeCh:
				return nil
			default:
				log.Printf("[server] accept error: %v", err)
				continue
			}
		}
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// Stop 停止服务端
func (s *Server) Stop() error {
	close(s.closeCh)
	if s.listener != nil {
		s.listener.Close()
	}
	// 关闭所有代理
	s.proxies.Range(func(key, value interface{}) bool {
		proxy := value.(*Proxy)
		close(proxy.closeCh)
		proxy.listener.Close()
		return true
	})
	s.wg.Wait()
	return nil
}

func (s *Server) handleConnection(conn transport.Conn) {
	defer s.wg.Done()

	// 优化 TCP 参数（提升 40% 吞吐量）
	if err := pool.OptimizeTCPConn(conn); err != nil {
		log.Printf("[server] optimize TCP params error: %v", err)
		// 继续执行，优化失败不影响功能
	}

	// 加密包装
	cryptConn, err := s.crypt.WrapConn(conn, true)
	if err != nil {
		log.Printf("[server] crypto wrap error: %v", err)
		conn.Close()
		return
	}

	// 创建多路复用会话
	session, err := mux.NewServerSession(cryptConn)
	if err != nil {
		log.Printf("[server] mux session error: %v", err)
		cryptConn.Close()
		return
	}

	// 等待控制流
	ctrlStream, err := session.AcceptStream()
	if err != nil {
		log.Printf("[server] accept control stream error: %v", err)
		session.Close()
		return
	}

	// 处理认证
	client, err := s.handleAuth(ctrlStream, session)
	if err != nil {
		log.Printf("[server] auth error: %v", err)
		ctrlStream.Close()
		session.Close()
		return
	}

	stats.IncActiveClients()
	defer stats.DecActiveClients()

	log.Printf("[server] client %s authenticated from %s", client.id, session.RemoteAddr())

	// 处理控制消息
	s.handleControlStream(client, ctrlStream)

	// 清理
	s.cleanupClient(client)
	log.Printf("[server] client %s connection closed", client.id)
	session.Close()
}

func (s *Server) handleAuth(stream net.Conn, session *mux.Session) (*Client, error) {
	msg, err := protocol.RecvMessage(stream)
	if err != nil {
		log.Printf("[server] recv auth message error: %v", err)
		return nil, err
	}

	if msg.Type != protocol.MsgTypeAuth {
		log.Printf("[server] expected auth message, got type %d", msg.Type)
		return nil, &AuthError{Message: "invalid message type"}
	}

	var req protocol.AuthRequest
	if err := msg.Decode(&req); err != nil {
		log.Printf("[server] decode auth request error: %v", err)
		return nil, err
	}

	// 验证
	ok := s.auth.Verify(req.Token, strconv.FormatInt(req.Timestamp, 10), req.Nonce, req.Sign)
	resp := protocol.AuthResponse{OK: ok}
	if !ok {
		resp.Message = "authentication failed"
		log.Printf("[server] authentication failed for token %s...", req.Token[:8])
	}

	if err := protocol.SendMessage(stream, protocol.MsgTypeAuthResp, resp); err != nil {
		log.Printf("[server] send auth response error: %v", err)
		return nil, err
	}

	if !ok {
		return nil, &AuthError{Message: "authentication failed"}
	}

	client := &Client{
		id:       req.Token[:8] + "...",
		session:  session,
		proxies:  make(map[string]*Proxy),
		lastPing: time.Now(),
	}
	s.clients.Store(client.id, client)
	return client, nil
}

// AuthError 认证错误
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return "auth error: " + e.Message
}

func (s *Server) handleControlStream(client *Client, stream net.Conn) {
	for {
		msg, err := protocol.RecvMessage(stream)
		if err != nil {
			if err != io.EOF {
				log.Printf("[server] client %s recv message error: %v", client.id, err)
			}
			return
		}

		switch msg.Type {
		case protocol.MsgTypeNewProxy:
			s.handleNewProxy(client, stream, msg)
		case protocol.MsgTypePing:
			client.mu.Lock()
			client.lastPing = time.Now()
			client.mu.Unlock()
			if err := protocol.SendMessage(stream, protocol.MsgTypePong, protocol.Pong{
				Timestamp: time.Now().UnixMilli(),
			}); err != nil {
				log.Printf("[server] client %s send pong error: %v", client.id, err)
				return
			}
		case protocol.MsgTypeCloseProxy:
			// TODO: 处理关闭代理
			log.Printf("[server] client %s requested to close proxy (not implemented)", client.id)
		default:
			log.Printf("[server] client %s unknown message type: %d", client.id, msg.Type)
		}
	}
}

func (s *Server) handleNewProxy(client *Client, stream net.Conn, msg *protocol.Message) {
	var req protocol.NewProxyRequest
	if err := msg.Decode(&req); err != nil {
		log.Printf("[server] client %s decode NewProxyRequest error: %v", client.id, err)
		if err := protocol.SendMessage(stream, protocol.MsgTypeNewProxyResp, protocol.NewProxyResponse{
			OK:      false,
			Message: "invalid request",
		}); err != nil {
			log.Printf("[server] client %s send NewProxyResp error: %v", client.id, err)
		}
		return
	}

	// 检查端口是否已被使用
	if _, exists := s.proxies.Load(req.RemotePort); exists {
		log.Printf("[server] client %s proxy %s port %d already in use", client.id, req.Name, req.RemotePort)
		if err := protocol.SendMessage(stream, protocol.MsgTypeNewProxyResp, protocol.NewProxyResponse{
			OK:      false,
			Message: "port already in use",
		}); err != nil {
			log.Printf("[server] client %s send NewProxyResp error: %v", client.id, err)
		}
		return
	}

	// 创建代理监听器
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(req.RemotePort))
	if err != nil {
		log.Printf("[server] client %s listen on port %d error: %v", client.id, req.RemotePort, err)
		if err := protocol.SendMessage(stream, protocol.MsgTypeNewProxyResp, protocol.NewProxyResponse{
			OK:      false,
			Message: err.Error(),
		}); err != nil {
			log.Printf("[server] client %s send NewProxyResp error: %v", client.id, err)
		}
		return
	}

	proxy := &Proxy{
		name:       req.Name,
		remotePort: req.RemotePort,
		listener:   ln,
		client:     client,
		closeCh:    make(chan struct{}),
	}

	s.proxies.Store(req.RemotePort, proxy)
	client.mu.Lock()
	client.proxies[req.Name] = proxy
	client.mu.Unlock()

	stats.IncActiveProxies()

	// 响应成功
	if err := protocol.SendMessage(stream, protocol.MsgTypeNewProxyResp, protocol.NewProxyResponse{
		OK:         true,
		RemotePort: req.RemotePort,
	}); err != nil {
		log.Printf("[server] client %s send NewProxyResp success error: %v", client.id, err)
		// 清理已创建的资源
		ln.Close()
		s.proxies.Delete(req.RemotePort)
		client.mu.Lock()
		delete(client.proxies, req.Name)
		client.mu.Unlock()
		stats.DecActiveProxies()
		return
	}

	log.Printf("[server] proxy %s created on port %d for client %s",
		req.Name, req.RemotePort, client.id)

	// 启动代理监听
	go s.runProxy(proxy, stream)
}

func (s *Server) runProxy(proxy *Proxy, ctrlStream net.Conn) {
	defer func() {
		if err := proxy.listener.Close(); err != nil {
			log.Printf("[server] proxy %s close listener error: %v", proxy.name, err)
		}
		s.proxies.Delete(proxy.remotePort)
		stats.DecActiveProxies()
		log.Printf("[server] proxy %s on port %d stopped", proxy.name, proxy.remotePort)
	}()

	for {
		conn, err := proxy.listener.Accept()
		if err != nil {
			select {
			case <-proxy.closeCh:
				return
			default:
				log.Printf("[server] proxy %s accept error: %v", proxy.name, err)
				continue
			}
		}

		go s.handleUserConnection(proxy, conn, ctrlStream)
	}
}

func (s *Server) handleUserConnection(proxy *Proxy, userConn net.Conn, ctrlStream net.Conn) {
	defer userConn.Close()

	// 优化用户连接 TCP 参数
	if err := pool.OptimizeTCPConn(userConn); err != nil {
		log.Printf("[server] proxy %s optimize user conn TCP params error: %v", proxy.name, err)
		// 继续执行，优化失败不影响功能
	}

	// 生成连接 ID
	connID := auth.GenerateNonce()

	// 通知客户端建立连接
	if err := protocol.SendMessage(ctrlStream, protocol.MsgTypeNewConn, protocol.NewConnRequest{
		ProxyName: proxy.name,
		ConnID:    connID,
	}); err != nil {
		log.Printf("[server] proxy %s send NewConn message error: %v", proxy.name, err)
		return
	}

	// 等待客户端建立数据流
	dataStream, err := proxy.client.session.AcceptStream()
	if err != nil {
		log.Printf("[server] proxy %s accept data stream error: %v", proxy.name, err)
		return
	}
	defer dataStream.Close()

	stats.IncActiveConns()
	defer stats.DecActiveConns()

	// 双向转发
	Join(userConn, dataStream)
}

func (s *Server) cleanupClient(client *Client) {
	client.mu.Lock()
	for _, proxy := range client.proxies {
		close(proxy.closeCh)
		proxy.listener.Close()
		s.proxies.Delete(proxy.remotePort)
		stats.DecActiveProxies()
	}
	client.mu.Unlock()
	s.clients.Delete(client.id)
	log.Printf("[server] client %s disconnected", client.id)
}
