package pool

import (
	"context"
	"net"
	"time"
)

func GetContext() context.Context {
	return context.Background()
}

// OptimizeTCPConn 优化 TCP 连接参数（适用于恶劣网络环境）
func OptimizeTCPConn(conn net.Conn) error {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		// 不是 TCP 连接，跳过优化
		return nil
	}

	// 1. 增大读写缓冲区（提高吞吐量，减少丢包影响）
	if err := tcpConn.SetReadBuffer(2 * 1024 * 1024); err != nil {
		// 非致命错误，记录但继续
		_ = err
	}

	if err := tcpConn.SetWriteBuffer(2 * 1024 * 1024); err != nil {
		_ = err
	}

	// 2. 禁用 Nagle 算法（降低延迟，适合控制连接）
	if err := tcpConn.SetNoDelay(true); err != nil {
		_ = err
	}

	// 3. 启用 TCP Keep-Alive（快速检测死连接）
	if err := tcpConn.SetKeepAlive(true); err != nil {
		return err
	}

	// 4. 设置 Keep-Alive 间隔（15秒，恶劣网络下更快检测断线）
	if err := tcpConn.SetKeepAlivePeriod(15 * time.Second); err != nil {
		return err
	}

	return nil
}
