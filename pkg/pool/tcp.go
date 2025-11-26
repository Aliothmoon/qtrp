package pool

import (
	"net"
)

func OptimizeTCPConn(conn net.Conn) error {
	//tcpConn, ok := conn.(*net.TCPConn)
	//if !ok {
	//	// 不是 TCP 连接，跳过优化
	//	return nil
	//}
	//
	//if err := tcpConn.SetReadBuffer(1024 * 1024); err != nil {
	//	return err
	//}
	//
	//if err := tcpConn.SetWriteBuffer(1024 * 1024); err != nil {
	//	return err
	//}
	//
	//// 4. 启用 Keep-Alive（检测死连接）
	//if err := tcpConn.SetKeepAlive(true); err != nil {
	//	return err
	//}
	//
	//// 5. Keep-Alive 间隔
	//if err := tcpConn.SetKeepAlivePeriod(30 * time.Second); err != nil {
	//	return err
	//}

	return nil
}
