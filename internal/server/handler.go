package server

import (
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"qtrp/pkg/pool"
	"qtrp/pkg/stats"
)

// isClosedConnError 判断是否是连接已关闭的错误（正常关闭场景）
func isClosedConnError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "use of closed network connection") ||
		strings.Contains(errStr, "connection reset by peer") ||
		strings.Contains(errStr, "broken pipe")
}

// Join 双向转发两个连接
func Join(c1, c2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer c1.Close()
		n, err := copyBuffer(c1, c2)
		if err != nil && err != io.EOF && !isClosedConnError(err) {
			log.Printf("[server] copy c2->c1 error after %d bytes: %v", n, err)
		}
	}()

	go func() {
		defer wg.Done()
		defer c2.Close()
		n, err := copyBuffer(c2, c1)
		if err != nil && err != io.EOF && !isClosedConnError(err) {
			log.Printf("[server] copy c1->c2 error after %d bytes: %v", n, err)
		}
	}()

	wg.Wait()
}

// copyBuffer 使用缓冲池进行数据复制
func copyBuffer(dst, src net.Conn) (int64, error) {
	buf := pool.DefaultBufferPool.Get()
	defer pool.DefaultBufferPool.Put(buf)

	var total int64
	for {
		n, err := src.Read(*buf)
		if n > 0 {
			stats.AddBytesIn(int64(n))

			wn, werr := dst.Write((*buf)[:n])
			if wn > 0 {
				total += int64(wn)
				stats.AddBytesOut(int64(wn))
			}
			if werr != nil {
				if !isClosedConnError(werr) {
					log.Printf("[server] write error after %d total bytes: %v", total, werr)
				}
				return total, werr
			}
			if wn != n {
				log.Printf("[server] short write: wrote %d of %d bytes", wn, n)
				return total, io.ErrShortWrite
			}
		}
		if err != nil {
			if err == io.EOF {
				return total, nil
			}
			if !isClosedConnError(err) {
				log.Printf("[server] read error after %d total bytes: %v", total, err)
			}
			return total, err
		}
	}
}
