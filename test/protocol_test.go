package test

import (
	"bytes"
	"testing"

	"qtrp/pkg/protocol"
)

// TestMessageEncodeDecode 测试消息编解码
func TestMessageEncodeDecode(t *testing.T) {
	tests := []struct {
		name    string
		msgType uint8
		payload interface{}
	}{
		{
			name:    "AuthRequest",
			msgType: protocol.MsgTypeAuth,
			payload: protocol.AuthRequest{
				Token:     "test-token",
				Timestamp: 1234567890,
				Nonce:     "random-nonce",
				Sign:      "signature",
				Version:   "1.0.0",
			},
		},
		{
			name:    "AuthResponse",
			msgType: protocol.MsgTypeAuthResp,
			payload: protocol.AuthResponse{
				OK:      true,
				Message: "success",
			},
		},
		{
			name:    "NewProxyRequest",
			msgType: protocol.MsgTypeNewProxy,
			payload: protocol.NewProxyRequest{
				Name:       "web",
				Type:       "tcp",
				RemotePort: 8080,
			},
		},
		{
			name:    "NewProxyResponse",
			msgType: protocol.MsgTypeNewProxyResp,
			payload: protocol.NewProxyResponse{
				OK:         true,
				RemotePort: 8080,
			},
		},
		{
			name:    "Ping",
			msgType: protocol.MsgTypePing,
			payload: protocol.Ping{Timestamp: 1234567890},
		},
		{
			name:    "Pong",
			msgType: protocol.MsgTypePong,
			payload: protocol.Pong{Timestamp: 1234567890},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 编码
			msg, err := protocol.NewMessage(tt.msgType, tt.payload)
			if err != nil {
				t.Fatalf("NewMessage failed: %v", err)
			}

			if msg.Type != tt.msgType {
				t.Errorf("expected type %d, got %d", tt.msgType, msg.Type)
			}

			// 序列化到 buffer
			var buf bytes.Buffer
			encoder := protocol.NewEncoder(&buf)
			if err := encoder.Encode(msg); err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// 反序列化
			decoder := protocol.NewDecoder(&buf)
			decoded, err := decoder.Decode()
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			if decoded.Type != tt.msgType {
				t.Errorf("decoded type mismatch: expected %d, got %d", tt.msgType, decoded.Type)
			}
		})
	}
}

// TestSendRecvMessage 测试便捷函数
func TestSendRecvMessage(t *testing.T) {
	var buf bytes.Buffer

	// 发送
	req := protocol.AuthRequest{
		Token:     "my-token",
		Timestamp: 9999,
		Nonce:     "nonce123",
		Sign:      "sig",
		Version:   "2.0",
	}
	if err := protocol.SendMessage(&buf, protocol.MsgTypeAuth, req); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// 接收
	msg, err := protocol.RecvMessage(&buf)
	if err != nil {
		t.Fatalf("RecvMessage failed: %v", err)
	}

	if msg.Type != protocol.MsgTypeAuth {
		t.Errorf("expected type %d, got %d", protocol.MsgTypeAuth, msg.Type)
	}

	// 解码 payload
	var decoded protocol.AuthRequest
	if err := msg.Decode(&decoded); err != nil {
		t.Fatalf("Decode payload failed: %v", err)
	}

	if decoded.Token != req.Token {
		t.Errorf("token mismatch: expected %s, got %s", req.Token, decoded.Token)
	}
	if decoded.Timestamp != req.Timestamp {
		t.Errorf("timestamp mismatch: expected %d, got %d", req.Timestamp, decoded.Timestamp)
	}
}

// TestLargePayload 测试大负载
func TestLargePayload(t *testing.T) {
	largeData := make([]byte, 100*1024) // 100KB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	payload := struct {
		Data []byte `json:"data"`
	}{Data: largeData}

	var buf bytes.Buffer
	if err := protocol.SendMessage(&buf, 100, payload); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	msg, err := protocol.RecvMessage(&buf)
	if err != nil {
		t.Fatalf("RecvMessage failed: %v", err)
	}

	var decoded struct {
		Data []byte `json:"data"`
	}
	if err := msg.Decode(&decoded); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !bytes.Equal(decoded.Data, largeData) {
		t.Error("large payload data mismatch")
	}

	t.Logf("Large payload test passed: %d bytes", len(largeData))
}
