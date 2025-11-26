package protocol

import (
	"github.com/vmihailenco/msgpack/v5"
)

// 消息类型
const (
	MsgTypeAuth         uint8 = 1 // 认证请求
	MsgTypeAuthResp     uint8 = 2 // 认证响应
	MsgTypeNewProxy     uint8 = 3 // 新建代理
	MsgTypeNewProxyResp uint8 = 4 // 新建代理响应
	MsgTypeNewConn      uint8 = 5 // 新建连接
	MsgTypeNewConnResp  uint8 = 6 // 新建连接响应
	MsgTypePing         uint8 = 7 // 心跳
	MsgTypePong         uint8 = 8 // 心跳响应
	MsgTypeCloseProxy   uint8 = 9 // 关闭代理
)

// AuthRequest 认证请求
type AuthRequest struct {
	Token     string `msgpack:"token"`
	Timestamp int64  `msgpack:"ts"`
	Nonce     string `msgpack:"nonce"`
	Sign      string `msgpack:"sign"`
	Version   string `msgpack:"version"`
}

// AuthResponse 认证响应
type AuthResponse struct {
	OK      bool   `msgpack:"ok"`
	Message string `msgpack:"message,omitempty"`
}

// NewProxyRequest 新建代理请求
type NewProxyRequest struct {
	Name       string `msgpack:"name"`
	Type       string `msgpack:"type"` // tcp
	RemotePort int    `msgpack:"remote_port"`
}

// NewProxyResponse 新建代理响应
type NewProxyResponse struct {
	OK         bool   `msgpack:"ok"`
	Message    string `msgpack:"message,omitempty"`
	RemotePort int    `msgpack:"remote_port"`
}

// NewConnRequest 新建连接请求（服务端 -> 客户端）
type NewConnRequest struct {
	ProxyName string `msgpack:"proxy_name"`
	ConnID    string `msgpack:"conn_id"`
}

// NewConnResponse 新建连接响应（客户端 -> 服务端）
type NewConnResponse struct {
	OK      bool   `msgpack:"ok"`
	ConnID  string `msgpack:"conn_id"`
	Message string `msgpack:"message,omitempty"`
}

// Ping 心跳
type Ping struct {
	Timestamp int64 `msgpack:"ts"`
}

// Pong 心跳响应
type Pong struct {
	Timestamp int64 `msgpack:"ts"`
}

// Message 通用消息包装
type Message struct {
	Type    uint8  `msgpack:"type"`
	Payload []byte `msgpack:"payload"`
}

// NewMessage 创建消息
func NewMessage(typ uint8, payload interface{}) (*Message, error) {
	data, err := msgpack.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Message{
		Type:    typ,
		Payload: data,
	}, nil
}

// Decode 解码消息载荷
func (m *Message) Decode(v interface{}) error {
	return msgpack.Unmarshal(m.Payload, v)
}
