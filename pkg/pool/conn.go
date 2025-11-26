package pool

import (
	"errors"
	"net"
	"sync"
	"time"
)

// ConnPool 连接池
type ConnPool struct {
	factory  func() (net.Conn, error)
	conns    chan net.Conn
	maxSize  int
	idleTime time.Duration
	mu       sync.Mutex
	closed   bool
}

// NewConnPool 创建连接池
func NewConnPool(factory func() (net.Conn, error), maxSize int) *ConnPool {
	return &ConnPool{
		factory:  factory,
		conns:    make(chan net.Conn, maxSize),
		maxSize:  maxSize,
		idleTime: 5 * time.Minute,
	}
}

// Get 获取连接
func (p *ConnPool) Get() (net.Conn, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, errors.New("pool closed")
	}
	p.mu.Unlock()

	// 尝试从池中获取
	select {
	case conn := <-p.conns:
		if p.isAlive(conn) {
			return conn, nil
		}
		conn.Close()
	default:
	}

	// 创建新连接
	return p.factory()
}

// Put 归还连接
func (p *ConnPool) Put(conn net.Conn) {
	if conn == nil {
		return
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		conn.Close()
		return
	}
	p.mu.Unlock()

	select {
	case p.conns <- conn:
	default:
		conn.Close()
	}
}

// Close 关闭连接池
func (p *ConnPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true

	close(p.conns)
	for conn := range p.conns {
		conn.Close()
	}
	return nil
}

// isAlive 检查连接是否存活
func (p *ConnPool) isAlive(conn net.Conn) bool {
	conn.SetReadDeadline(time.Now().Add(time.Millisecond))
	buf := make([]byte, 1)
	_, err := conn.Read(buf)
	conn.SetReadDeadline(time.Time{})

	if err == nil {
		return true
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	return false
}

// Size 返回池中连接数
func (p *ConnPool) Size() int {
	return len(p.conns)
}
