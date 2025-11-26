package crypto

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
)

// ChaCha20Crypto ChaCha20-Poly1305 加密实现
type ChaCha20Crypto struct {
	config *Config
}

// NewChaCha20Crypto 创建 ChaCha20 加密层
func NewChaCha20Crypto(cfg *Config) *ChaCha20Crypto {
	return &ChaCha20Crypto{config: cfg}
}

func (c *ChaCha20Crypto) Type() string {
	return "chacha20"
}

// WrapConn 包装连接为加密连接
func (c *ChaCha20Crypto) WrapConn(conn net.Conn, isServer bool) (net.Conn, error) {
	// 从 PSK 派生密钥
	key := deriveKey(c.config.PSK)

	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}

	return &ChaCha20Conn{
		Conn: conn,
		aead: aead,
	}, nil
}

// WrapListener ChaCha20 不需要包装监听器
func (c *ChaCha20Crypto) WrapListener(ln net.Listener) (net.Listener, error) {
	return ln, nil
}

// deriveKey 从 PSK 派生 32 字节密钥
func deriveKey(psk string) []byte {
	hash := sha256.Sum256([]byte(psk))
	return hash[:]
}

// ChaCha20Conn 加密连接
type ChaCha20Conn struct {
	net.Conn
	aead    cipher.AEAD
	readMu  sync.Mutex
	writeMu sync.Mutex
	readBuf []byte
}

// 帧格式: [2字节长度][nonce][密文+tag]
const (
	lengthSize = 2
	nonceSize  = 12
	tagSize    = 16
	maxPayload = 65536 - nonceSize - tagSize - lengthSize // ~64KB 载荷，提升吞吐
)

// 缓冲池复用，减少 GC
var (
	framePool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, lengthSize+nonceSize+maxPayload+tagSize)
			return &buf
		},
	}
	readPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 65536)
			return &buf
		},
	}
)

func (c *ChaCha20Conn) Write(b []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	total := 0
	for len(b) > 0 {
		chunk := b
		if len(chunk) > maxPayload {
			chunk = b[:maxPayload]
		}
		b = b[len(chunk):]

		if err := c.writeFrame(chunk); err != nil {
			return total, err
		}
		total += len(chunk)
	}
	return total, nil
}

func (c *ChaCha20Conn) writeFrame(payload []byte) error {
	// 从池中获取缓冲区
	bufPtr := framePool.Get().(*[]byte)
	defer framePool.Put(bufPtr)
	frame := *bufPtr

	// 生成随机 nonce
	nonce := frame[lengthSize : lengthSize+nonceSize]
	if _, err := rand.Read(nonce); err != nil {
		return err
	}

	// 加密（直接写入 frame 避免分配）
	ciphertext := c.aead.Seal(frame[lengthSize+nonceSize:lengthSize+nonceSize], nonce, payload, nil)

	// 写入长度
	frameLen := nonceSize + len(ciphertext)
	binary.BigEndian.PutUint16(frame[:lengthSize], uint16(frameLen))

	// 发送
	_, err := c.Conn.Write(frame[:lengthSize+frameLen])
	return err
}

func (c *ChaCha20Conn) Read(b []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	// 先消费缓冲区
	if len(c.readBuf) > 0 {
		n := copy(b, c.readBuf)
		c.readBuf = c.readBuf[n:]
		return n, nil
	}

	// 读取新帧
	plaintext, err := c.readFrame()
	if err != nil {
		return 0, err
	}

	n := copy(b, plaintext)
	if n < len(plaintext) {
		c.readBuf = plaintext[n:]
	}
	return n, nil
}

func (c *ChaCha20Conn) readFrame() ([]byte, error) {
	// 读取长度
	lenBuf := make([]byte, lengthSize)
	if _, err := io.ReadFull(c.Conn, lenBuf); err != nil {
		return nil, err
	}
	frameLen := binary.BigEndian.Uint16(lenBuf)

	if frameLen < nonceSize+tagSize {
		return nil, errors.New("invalid frame length")
	}

	// 从池中获取读缓冲
	bufPtr := readPool.Get().(*[]byte)
	defer readPool.Put(bufPtr)
	frame := (*bufPtr)[:frameLen]

	if _, err := io.ReadFull(c.Conn, frame); err != nil {
		return nil, err
	}

	// 解密
	nonce := frame[:nonceSize]
	ciphertext := frame[nonceSize:]

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
