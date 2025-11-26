package test

import (
	"bytes"
	"crypto/rand"
	"io"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"qtrp/pkg/transport"
)

// TestNetTransport 测试标准库 net 传输
func TestNetTransport(t *testing.T) {
	trans := transport.NewNetTransport()
	testTransport(t, trans, "net")
}

// TestGnetTransport 测试 gnet 传输
func TestGnetTransport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("gnet tests are skipped on Windows due to platform limitations")
	}
	trans := transport.NewGnetTransport()
	testTransport(t, trans, "gnet")
}

// TestQUICTransport 测试 QUIC 传输
func TestQUICTransport(t *testing.T) {
	trans := transport.NewQUICTransport()
	testTransport(t, trans, "quic")
}

// testTransport 通用传输层测试
func testTransport(t *testing.T, trans transport.Transport, name string) {
	t.Run(name+"_Type", func(t *testing.T) {
		if trans.Type() != name {
			t.Errorf("expected type %s, got %s", name, trans.Type())
		}
	})

	t.Run(name+"_ListenAndDial", func(t *testing.T) {
		testListenAndDial(t, trans)
	})

	t.Run(name+"_EchoSmall", func(t *testing.T) {
		testEcho(t, trans, 1024) // 1KB
	})

	t.Run(name+"_EchoMedium", func(t *testing.T) {
		testEcho(t, trans, 64*1024) // 64KB
	})

	t.Run(name+"_EchoLarge", func(t *testing.T) {
		testEcho(t, trans, 1024*1024) // 1MB
	})

	t.Run(name+"_MultipleConnections", func(t *testing.T) {
		testMultipleConnections(t, trans, 10)
	})

	t.Run(name+"_ConcurrentReadWrite", func(t *testing.T) {
		testConcurrentReadWrite(t, trans)
	})
}

// testListenAndDial 测试监听和连接
func testListenAndDial(t *testing.T, trans transport.Transport) {
	ln, err := trans.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	t.Logf("Listening on %s", addr)

	// 异步接受连接
	serverConnCh := make(chan transport.Conn, 1)
	serverErrCh := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			serverErrCh <- err
			return
		}
		serverConnCh <- conn
	}()

	// 客户端连接
	clientConn, err := trans.Dial(addr)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer clientConn.Close()

	// 等待服务端接受连接
	select {
	case serverConn := <-serverConnCh:
		defer serverConn.Close()
		t.Logf("Connection established: client=%s, server=%s",
			clientConn.LocalAddr(), serverConn.RemoteAddr())
	case err := <-serverErrCh:
		t.Fatalf("Accept failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Accept timeout")
	}
}

// testEcho 测试数据回显
func testEcho(t *testing.T, trans transport.Transport, size int) {
	ln, err := trans.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()

	// 生成随机数据
	sendData := make([]byte, size)
	rand.Read(sendData)

	// 服务端：回显数据
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := ln.Accept()
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer conn.Close()

		// 回显所有数据
		buf := make([]byte, 32*1024)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				if err != io.EOF {
					// 可能是连接关闭
				}
				return
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				return
			}
		}
	}()

	// 客户端连接并发送数据
	clientConn, err := trans.Dial(addr)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}

	// 发送数据
	go func() {
		clientConn.Write(sendData)
		// 给服务端时间回显
		time.Sleep(100 * time.Millisecond)
		clientConn.Close()
	}()

	// 接收回显数据
	recvData := make([]byte, size)
	totalRead := 0
	clientConn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for totalRead < size {
		n, err := clientConn.Read(recvData[totalRead:])
		if err != nil {
			if err == io.EOF {
				break
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			t.Fatalf("Read failed: %v", err)
		}
		totalRead += n
	}

	wg.Wait()

	// 验证数据
	if totalRead != size {
		t.Errorf("Data size mismatch: sent %d, received %d", size, totalRead)
		return
	}
	if !bytes.Equal(sendData, recvData) {
		t.Error("Data content mismatch")
	}

	t.Logf("Echo test passed: %d bytes", size)
}

// testMultipleConnections 测试多连接
func testMultipleConnections(t *testing.T, trans transport.Transport, count int) {
	ln, err := trans.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()

	var wg sync.WaitGroup
	errors := make(chan error, count*2)

	// 服务端接受多个连接
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			conn, err := ln.Accept()
			if err != nil {
				errors <- err
				return
			}
			go func(c transport.Conn) {
				defer c.Close()
				io.Copy(c, c) // 回显
			}(conn)
		}
	}()

	// 客户端建立多个连接
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			conn, err := trans.Dial(addr)
			if err != nil {
				errors <- err
				return
			}
			defer conn.Close()

			// 发送并验证数据
			msg := []byte("hello from client")
			if _, err := conn.Write(msg); err != nil {
				errors <- err
				return
			}

			buf := make([]byte, len(msg))
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			if _, err := io.ReadFull(conn, buf); err != nil {
				errors <- err
				return
			}

			if !bytes.Equal(msg, buf) {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Errorf("Error: %v", err)
		}
	}

	t.Logf("Multiple connections test passed: %d connections", count)
}

// testConcurrentReadWrite 测试并发读写
func testConcurrentReadWrite(t *testing.T, trans transport.Transport) {
	ln, err := trans.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	msgCount := 100
	msgSize := 1024

	var wg sync.WaitGroup
	wg.Add(2)

	// 服务端
	go func() {
		defer wg.Done()
		conn, err := ln.Accept()
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer conn.Close()

		// 同时读写
		var innerWg sync.WaitGroup
		innerWg.Add(2)

		// 读
		go func() {
			defer innerWg.Done()
			buf := make([]byte, msgSize)
			for i := 0; i < msgCount; i++ {
				if _, err := io.ReadFull(conn, buf); err != nil {
					return
				}
			}
		}()

		// 写
		go func() {
			defer innerWg.Done()
			data := make([]byte, msgSize)
			for i := 0; i < msgCount; i++ {
				if _, err := conn.Write(data); err != nil {
					return
				}
			}
		}()

		innerWg.Wait()
	}()

	// 客户端
	go func() {
		defer wg.Done()
		conn, err := trans.Dial(addr)
		if err != nil {
			t.Errorf("Dial failed: %v", err)
			return
		}
		defer conn.Close()

		var innerWg sync.WaitGroup
		innerWg.Add(2)

		// 写
		go func() {
			defer innerWg.Done()
			data := make([]byte, msgSize)
			for i := 0; i < msgCount; i++ {
				if _, err := conn.Write(data); err != nil {
					return
				}
			}
		}()

		// 读
		go func() {
			defer innerWg.Done()
			buf := make([]byte, msgSize)
			for i := 0; i < msgCount; i++ {
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				if _, err := io.ReadFull(conn, buf); err != nil {
					return
				}
			}
		}()

		innerWg.Wait()
	}()

	wg.Wait()
	t.Logf("Concurrent read/write test passed: %d messages of %d bytes", msgCount, msgSize)
}
