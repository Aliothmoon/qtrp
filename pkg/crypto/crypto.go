package crypto

import (
	"net"
)

// Config 加密配置
type Config struct {
	// TLS 配置
	CertFile   string
	KeyFile    string
	CACert     string
	SkipVerify bool

	// ChaCha20 配置
	PSK string // 预共享密钥
}

// Crypto 加密层接口，支持 TLS/ChaCha20 切换
type Crypto interface {
	// WrapConn 将连接包装为加密连接
	WrapConn(conn net.Conn, isServer bool) (net.Conn, error)
	// WrapListener 包装监听器 (仅 TLS 使用)
	WrapListener(ln net.Listener) (net.Listener, error)
	// Type 返回加密类型标识
	Type() string
}

// New 创建加密层实例
func New(typ string, cfg *Config) Crypto {
	switch typ {
	case "chacha20":
		return NewChaCha20Crypto(cfg)
	case "none":
		return NewNoneCrypto()
	default:
		return NewTLSCrypto(cfg)
	}
}

// NoneCrypto 无加密（用于测试）
type NoneCrypto struct{}

func NewNoneCrypto() *NoneCrypto {
	return &NoneCrypto{}
}

func (n *NoneCrypto) WrapConn(conn net.Conn, isServer bool) (net.Conn, error) {
	return conn, nil
}

func (n *NoneCrypto) WrapListener(ln net.Listener) (net.Listener, error) {
	return ln, nil
}

func (n *NoneCrypto) Type() string {
	return "none"
}
