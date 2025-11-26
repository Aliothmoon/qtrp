package pool

import (
	"sync"
)

// BufferPool 缓冲区池
type BufferPool struct {
	pool sync.Pool
	size int
}

// NewBufferPool 创建缓冲区池
func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		size: size,
		pool: sync.Pool{
			New: func() interface{} {
				buf := make([]byte, size)
				return &buf
			},
		},
	}
}

// Get 获取缓冲区
func (p *BufferPool) Get() *[]byte {
	return p.pool.Get().(*[]byte)
}

// Put 归还缓冲区
func (p *BufferPool) Put(buf *[]byte) {
	if buf != nil && len(*buf) == p.size {
		p.pool.Put(buf)
	}
}

var DefaultBufferPool = NewBufferPool(512 * 1024)
