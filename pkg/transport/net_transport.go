package transport

import (
	"net"
)

// NetTransport 标准库 net 实现
type NetTransport struct{}

// NewNetTransport 创建 net 传输层
func NewNetTransport() *NetTransport {
	return &NetTransport{}
}

func (t *NetTransport) Type() string {
	return "net"
}

func (t *NetTransport) Listen(addr string) (Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &NetListener{ln: ln}, nil
}

func (t *NetTransport) Dial(addr string) (Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &NetConn{Conn: conn}, nil
}

// NetListener 标准库监听器包装
type NetListener struct {
	ln net.Listener
}

func (l *NetListener) Accept() (Conn, error) {
	conn, err := l.ln.Accept()
	if err != nil {
		return nil, err
	}
	return &NetConn{Conn: conn}, nil
}

func (l *NetListener) Close() error {
	return l.ln.Close()
}

func (l *NetListener) Addr() net.Addr {
	return l.ln.Addr()
}

// Listener 返回底层 net.Listener
func (l *NetListener) Listener() net.Listener {
	return l.ln
}

// NetConn 标准库连接包装
type NetConn struct {
	net.Conn
	ctx interface{}
}

func (c *NetConn) SetContext(ctx interface{}) {
	c.ctx = ctx
}

func (c *NetConn) Context() interface{} {
	return c.ctx
}

// WrapNetConn 包装 net.Conn 为 Conn
func WrapNetConn(conn net.Conn) Conn {
	if c, ok := conn.(Conn); ok {
		return c
	}
	return &NetConn{Conn: conn}
}
