package stats

import (
	"sync/atomic"
	"time"
)

// Stats 全局统计信息
type Stats struct {
	// 连接统计
	ActiveClients int64 `json:"active_clients"`
	ActiveProxies int64 `json:"active_proxies"`
	ActiveConns   int64 `json:"active_conns"`
	TotalConns    int64 `json:"total_conns"`

	// 流量统计
	BytesIn  int64 `json:"bytes_in"`
	BytesOut int64 `json:"bytes_out"`

	// 时间
	StartTime time.Time `json:"start_time"`
	Uptime    string    `json:"uptime"`
}

var globalStats = &Stats{StartTime: time.Now()}

// AddBytesIn 增加入站流量
func AddBytesIn(n int64) {
	atomic.AddInt64(&globalStats.BytesIn, n)
}

// AddBytesOut 增加出站流量
func AddBytesOut(n int64) {
	atomic.AddInt64(&globalStats.BytesOut, n)
}

// IncActiveClients 增加活跃客户端数
func IncActiveClients() {
	atomic.AddInt64(&globalStats.ActiveClients, 1)
}

// DecActiveClients 减少活跃客户端数
func DecActiveClients() {
	atomic.AddInt64(&globalStats.ActiveClients, -1)
}

// IncActiveProxies 增加活跃代理数
func IncActiveProxies() {
	atomic.AddInt64(&globalStats.ActiveProxies, 1)
}

// DecActiveProxies 减少活跃代理数
func DecActiveProxies() {
	atomic.AddInt64(&globalStats.ActiveProxies, -1)
}

// IncActiveConns 增加活跃连接数
func IncActiveConns() {
	atomic.AddInt64(&globalStats.ActiveConns, 1)
	atomic.AddInt64(&globalStats.TotalConns, 1)
}

// DecActiveConns 减少活跃连接数
func DecActiveConns() {
	atomic.AddInt64(&globalStats.ActiveConns, -1)
}

// Get 获取统计信息快照
func Get() Stats {
	s := Stats{
		ActiveClients: atomic.LoadInt64(&globalStats.ActiveClients),
		ActiveProxies: atomic.LoadInt64(&globalStats.ActiveProxies),
		ActiveConns:   atomic.LoadInt64(&globalStats.ActiveConns),
		TotalConns:    atomic.LoadInt64(&globalStats.TotalConns),
		BytesIn:       atomic.LoadInt64(&globalStats.BytesIn),
		BytesOut:      atomic.LoadInt64(&globalStats.BytesOut),
		StartTime:     globalStats.StartTime,
	}
	s.Uptime = time.Since(globalStats.StartTime).String()
	return s
}

// CountingReader 带统计的 Reader
type CountingReader struct {
	r     interface{ Read([]byte) (int, error) }
	count *int64
}

func NewCountingReader(r interface{ Read([]byte) (int, error) }) *CountingReader {
	return &CountingReader{r: r, count: &globalStats.BytesIn}
}

func (c *CountingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		atomic.AddInt64(c.count, int64(n))
	}
	return n, err
}

// CountingWriter 带统计的 Writer
type CountingWriter struct {
	w     interface{ Write([]byte) (int, error) }
	count *int64
}

func NewCountingWriter(w interface{ Write([]byte) (int, error) }) *CountingWriter {
	return &CountingWriter{w: w, count: &globalStats.BytesOut}
}

func (c *CountingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	if n > 0 {
		atomic.AddInt64(c.count, int64(n))
	}
	return n, err
}
