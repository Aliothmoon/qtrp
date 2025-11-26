package transport

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/panjf2000/gnet/v2"
)

// GnetTransport gnet 实现
type GnetTransport struct{}

// NewGnetTransport 创建 gnet 传输层
func NewGnetTransport() *GnetTransport {
	return &GnetTransport{}
}

func (t *GnetTransport) Type() string {
	return "gnet"
}

func (t *GnetTransport) Listen(addr string) (Listener, error) {
	ln := &GnetListener{
		addr:     addr,
		acceptCh: make(chan *GnetConn, 1024),
		closeCh:  make(chan struct{}),
	}

	// 启动 gnet 事件循环
	go func() {
		err := gnet.Run(ln, "tcp://"+addr,
			gnet.WithMulticore(true),
			gnet.WithReusePort(true),
			gnet.WithTCPNoDelay(gnet.TCPNoDelay),
		)
		if err != nil && !errors.Is(err, context.Canceled) {
			// log error
		}
	}()

	// 等待启动
	time.Sleep(100 * time.Millisecond)
	return ln, nil
}

func (t *GnetTransport) Dial(addr string) (Conn, error) {
	// gnet 主要用于服务端，客户端使用标准 net
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &NetConn{Conn: conn}, nil
}

// GnetListener gnet 监听器
type GnetListener struct {
	gnet.BuiltinEventEngine
	addr     string
	acceptCh chan *GnetConn
	closeCh  chan struct{}
	engine   gnet.Engine
	closed   bool
	mu       sync.Mutex
}

func (l *GnetListener) OnBoot(eng gnet.Engine) gnet.Action {
	l.engine = eng
	return gnet.None
}

func (l *GnetListener) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	conn := &GnetConn{
		gnetConn: c,
		readCh:   make(chan []byte, 256),
		closeCh:  make(chan struct{}),
	}
	c.SetContext(conn)

	select {
	case l.acceptCh <- conn:
	case <-l.closeCh:
		return nil, gnet.Close
	}
	return nil, gnet.None
}

func (l *GnetListener) OnClose(c gnet.Conn, err error) gnet.Action {
	if conn, ok := c.Context().(*GnetConn); ok {
		conn.closeOnce.Do(func() {
			close(conn.closeCh)
		})
	}
	return gnet.None
}

func (l *GnetListener) OnTraffic(c gnet.Conn) gnet.Action {
	conn, ok := c.Context().(*GnetConn)
	if !ok {
		return gnet.Close
	}

	// 读取所有可用数据
	buf, _ := c.Next(-1)
	if len(buf) > 0 {
		data := make([]byte, len(buf))
		copy(data, buf)
		select {
		case conn.readCh <- data:
		case <-conn.closeCh:
			return gnet.Close
		default:
			// 缓冲区满，丢弃数据（生产环境应该处理）
		}
	}
	return gnet.None
}

func (l *GnetListener) Accept() (Conn, error) {
	select {
	case conn := <-l.acceptCh:
		return conn, nil
	case <-l.closeCh:
		return nil, errors.New("listener closed")
	}
}

func (l *GnetListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	close(l.closeCh)
	if l.engine.Validate() == nil {
		return l.engine.Stop(context.Background())
	}
	return nil
}

func (l *GnetListener) Addr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", l.addr)
	return addr
}

// GnetConn gnet 连接适配器，实现 net.Conn 接口
type GnetConn struct {
	gnetConn  gnet.Conn
	readCh    chan []byte
	readBuf   []byte
	closeCh   chan struct{}
	closeOnce sync.Once
	ctx       interface{}
	mu        sync.Mutex
}

func (c *GnetConn) Read(b []byte) (int, error) {
	// 先消费缓冲区
	if len(c.readBuf) > 0 {
		n := copy(b, c.readBuf)
		c.readBuf = c.readBuf[n:]
		return n, nil
	}

	// 从 channel 读取
	select {
	case data := <-c.readCh:
		n := copy(b, data)
		if n < len(data) {
			c.readBuf = data[n:]
		}
		return n, nil
	case <-c.closeCh:
		return 0, io.EOF
	}
}

func (c *GnetConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-c.closeCh:
		return 0, errors.New("connection closed")
	default:
	}

	return c.gnetConn.Write(b)
}

func (c *GnetConn) Close() error {
	c.closeOnce.Do(func() {
		close(c.closeCh)
	})
	return c.gnetConn.Close()
}

func (c *GnetConn) LocalAddr() net.Addr {
	return c.gnetConn.LocalAddr()
}

func (c *GnetConn) RemoteAddr() net.Addr {
	return c.gnetConn.RemoteAddr()
}

func (c *GnetConn) SetDeadline(t time.Time) error {
	return c.gnetConn.SetDeadline(t)
}

func (c *GnetConn) SetReadDeadline(t time.Time) error {
	return c.gnetConn.SetReadDeadline(t)
}

func (c *GnetConn) SetWriteDeadline(t time.Time) error {
	return c.gnetConn.SetWriteDeadline(t)
}

func (c *GnetConn) SetContext(ctx interface{}) {
	c.ctx = ctx
}

func (c *GnetConn) Context() interface{} {
	return c.ctx
}
