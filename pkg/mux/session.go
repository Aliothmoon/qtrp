package mux

import (
	"net"
	"time"

	"github.com/xtaci/smux"
)

// Session smux 会话封装
type Session struct {
	session *smux.Session
	conn    net.Conn
}

// DefaultConfig 默认 smux 配置（针对高带宽优化）
func DefaultConfig() *smux.Config {
	cfg := smux.DefaultConfig()
	cfg.Version = 2                          // 使用 v2 协议（性能更好）
	cfg.KeepAliveInterval = 30 * time.Second // 保活间隔
	cfg.KeepAliveTimeout = 90 * time.Second  // 保活超时
	cfg.MaxFrameSize = 65535                 // 最大帧大小（smux 限制）
	cfg.MaxReceiveBuffer = 16 * 1024 * 1024  // 16MB 接收缓冲（支持大窗口）
	cfg.MaxStreamBuffer = 2 * 1024 * 1024    // 2MB 流缓冲（关键：提升单流带宽）
	return cfg
}

// NewServerSession 创建服务端会话
func NewServerSession(conn net.Conn) (*Session, error) {
	session, err := smux.Server(conn, DefaultConfig())
	if err != nil {
		return nil, err
	}
	return &Session{
		session: session,
		conn:    conn,
	}, nil
}

// NewClientSession 创建客户端会话
func NewClientSession(conn net.Conn) (*Session, error) {
	session, err := smux.Client(conn, DefaultConfig())
	if err != nil {
		return nil, err
	}
	return &Session{
		session: session,
		conn:    conn,
	}, nil
}

// AcceptStream 接受新的流
func (s *Session) AcceptStream() (net.Conn, error) {
	stream, err := s.session.AcceptStream()
	if err != nil {
		return nil, err
	}
	return stream, nil
}

// OpenStream 打开新的流
func (s *Session) OpenStream() (net.Conn, error) {
	stream, err := s.session.OpenStream()
	if err != nil {
		return nil, err
	}
	return stream, nil
}

// Close 关闭会话
func (s *Session) Close() error {
	return s.session.Close()
}

// IsClosed 检查会话是否已关闭
func (s *Session) IsClosed() bool {
	return s.session.IsClosed()
}

// NumStreams 返回活跃流数量
func (s *Session) NumStreams() int {
	return s.session.NumStreams()
}

// Ping 发送 ping（smux 不支持 Ping，返回 0 延迟）
func (s *Session) Ping() (time.Duration, error) {
	// smux 没有内置 Ping 方法，可通过保活机制检测连接
	// 这里返回 0 表示不支持
	return 0, nil
}

// RemoteAddr 返回远程地址
func (s *Session) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

// LocalAddr 返回本地地址
func (s *Session) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}
