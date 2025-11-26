package transport

import (
	"net"
)

// Transport 传输层接口，支持 net/gnet 切换
type Transport interface {
	// Listen 创建监听器
	Listen(addr string) (Listener, error)
	// Dial 创建连接
	Dial(addr string) (Conn, error)
	// Type 返回传输类型标识
	Type() string
}

// Listener 监听器接口
type Listener interface {
	// Accept 接受新连接
	Accept() (Conn, error)
	// Close 关闭监听器
	Close() error
	// Addr 返回监听地址
	Addr() net.Addr
}

// Conn 连接接口，扩展 net.Conn
type Conn interface {
	net.Conn
	// SetContext 设置上下文 (gnet 需要)
	SetContext(ctx interface{})
	// Context 获取上下文
	Context() interface{}
}

// New 创建传输层实例
func New(typ string) Transport {
	switch typ {
	case "gnet":
		return NewGnetTransport()
	case "quic":
		return NewQUICTransport()
	default:
		return NewNetTransport()
	}
}

// NewWithQUICConfig 创建带 QUIC 配置的传输层
func NewWithQUICConfig(typ string, quicCfg *QUICConfig) Transport {
	switch typ {
	case "gnet":
		return NewGnetTransport()
	case "quic":
		return NewQUICTransportWithConfig(quicCfg)
	default:
		return NewNetTransport()
	}
}
