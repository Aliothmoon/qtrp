package protocol

import (
	"encoding/binary"
	"io"

	"github.com/vmihailenco/msgpack/v5"
)

// 帧格式: [4字节长度][msgpack数据]
const (
	LengthSize = 4
	MaxMsgSize = 1024 * 1024 // 1MB
)

// Encoder 消息编码器
type Encoder struct {
	w io.Writer
}

// NewEncoder 创建编码器
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Encode 编码并发送消息
func (e *Encoder) Encode(msg *Message) error {
	data, err := msgpack.Marshal(msg)
	if err != nil {
		return err
	}

	// 写入长度
	lenBuf := make([]byte, LengthSize)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(data)))
	if _, err := e.w.Write(lenBuf); err != nil {
		return err
	}

	// 写入数据
	_, err = e.w.Write(data)
	return err
}

// Decoder 消息解码器
type Decoder struct {
	r io.Reader
}

// NewDecoder 创建解码器
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// Decode 读取并解码消息
func (d *Decoder) Decode() (*Message, error) {
	// 读取长度
	lenBuf := make([]byte, LengthSize)
	if _, err := io.ReadFull(d.r, lenBuf); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)

	if length > MaxMsgSize {
		return nil, io.ErrShortBuffer
	}

	// 读取数据
	data := make([]byte, length)
	if _, err := io.ReadFull(d.r, data); err != nil {
		return nil, err
	}

	var msg Message
	if err := msgpack.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// SendMessage 发送消息的便捷函数
func SendMessage(w io.Writer, typ uint8, payload interface{}) error {
	msg, err := NewMessage(typ, payload)
	if err != nil {
		return err
	}
	return NewEncoder(w).Encode(msg)
}

// RecvMessage 接收消息的便捷函数
func RecvMessage(r io.Reader) (*Message, error) {
	return NewDecoder(r).Decode()
}
