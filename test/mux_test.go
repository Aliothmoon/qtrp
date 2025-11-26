package test

import (
	"bytes"
	"crypto/rand"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"qtrp/pkg/mux"
)

// TestMuxSession 测试 smux 会话
func TestMuxSession(t *testing.T) {
	server, client := createConnPair(t)

	serverSession, err := mux.NewServerSession(server)
	if err != nil {
		t.Fatalf("NewServerSession failed: %v", err)
	}
	defer serverSession.Close()

	clientSession, err := mux.NewClientSession(client)
	if err != nil {
		t.Fatalf("NewClientSession failed: %v", err)
	}
	defer clientSession.Close()

	t.Run("SingleStream", func(t *testing.T) {
		testSingleStream(t, serverSession, clientSession)
	})

	t.Run("MultipleStreams", func(t *testing.T) {
		testMultipleStreams(t, serverSession, clientSession, 10)
	})

	t.Run("BidirectionalData", func(t *testing.T) {
		testBidirectionalData(t, serverSession, clientSession)
	})

	t.Run("Ping", func(t *testing.T) {
		// smux 不支持真正的 Ping，返回 0 延迟
		rtt, err := clientSession.Ping()
		if err != nil {
			t.Fatalf("Ping failed: %v", err)
		}
		t.Logf("Ping RTT: %v (smux uses keepalive instead)", rtt)
	})
}

// testSingleStream 测试单流传输
func testSingleStream(t *testing.T, server, client *mux.Session) {
	var wg sync.WaitGroup
	wg.Add(2)

	data := []byte("hello from client")

	// 服务端接受流
	go func() {
		defer wg.Done()
		stream, err := server.AcceptStream()
		if err != nil {
			t.Errorf("AcceptStream failed: %v", err)
			return
		}
		defer stream.Close()

		buf := make([]byte, len(data))
		if _, err := io.ReadFull(stream, buf); err != nil {
			t.Errorf("Read failed: %v", err)
			return
		}

		if !bytes.Equal(buf, data) {
			t.Errorf("data mismatch")
		}

		// 回显
		stream.Write(buf)
	}()

	// 客户端打开流
	go func() {
		defer wg.Done()
		stream, err := client.OpenStream()
		if err != nil {
			t.Errorf("OpenStream failed: %v", err)
			return
		}
		defer stream.Close()

		if _, err := stream.Write(data); err != nil {
			t.Errorf("Write failed: %v", err)
			return
		}

		buf := make([]byte, len(data))
		if _, err := io.ReadFull(stream, buf); err != nil {
			t.Errorf("Read failed: %v", err)
			return
		}

		if !bytes.Equal(buf, data) {
			t.Errorf("echo data mismatch")
		}
	}()

	wg.Wait()
}

// testMultipleStreams 测试多流并发
func testMultipleStreams(t *testing.T, server, client *mux.Session, count int) {
	var wg sync.WaitGroup

	// 服务端处理多个流
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			stream, err := server.AcceptStream()
			if err != nil {
				t.Errorf("AcceptStream %d failed: %v", i, err)
				return
			}
			go func(s net.Conn) {
				defer s.Close()
				io.Copy(s, s) // 回显
			}(stream)
		}
	}()

	// 客户端创建多个流
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			stream, err := client.OpenStream()
			if err != nil {
				t.Errorf("OpenStream %d failed: %v", idx, err)
				return
			}
			defer stream.Close()

			// 发送数据
			data := make([]byte, 1024)
			rand.Read(data)
			stream.Write(data)

			// 读取回显
			buf := make([]byte, 1024)
			stream.SetReadDeadline(time.Now().Add(5 * time.Second))
			if _, err := io.ReadFull(stream, buf); err != nil {
				t.Errorf("Stream %d read failed: %v", idx, err)
				return
			}

			if !bytes.Equal(data, buf) {
				t.Errorf("Stream %d data mismatch", idx)
			}
		}(i)
	}

	wg.Wait()
	t.Logf("Multiple streams test passed: %d streams", count)
}

// testBidirectionalData 测试双向数据传输
func testBidirectionalData(t *testing.T, server, client *mux.Session) {
	var wg sync.WaitGroup
	wg.Add(2)

	msgCount := 50
	msgSize := 4096

	// 服务端
	go func() {
		defer wg.Done()
		stream, _ := server.AcceptStream()
		defer stream.Close()

		var innerWg sync.WaitGroup
		innerWg.Add(2)

		// 读
		go func() {
			defer innerWg.Done()
			buf := make([]byte, msgSize)
			for i := 0; i < msgCount; i++ {
				if _, err := io.ReadFull(stream, buf); err != nil {
					return
				}
			}
		}()

		// 写
		go func() {
			defer innerWg.Done()
			data := make([]byte, msgSize)
			for i := 0; i < msgCount; i++ {
				stream.Write(data)
			}
		}()

		innerWg.Wait()
	}()

	// 客户端
	go func() {
		defer wg.Done()
		stream, _ := client.OpenStream()
		defer stream.Close()

		var innerWg sync.WaitGroup
		innerWg.Add(2)

		// 写
		go func() {
			defer innerWg.Done()
			data := make([]byte, msgSize)
			for i := 0; i < msgCount; i++ {
				stream.Write(data)
			}
		}()

		// 读
		go func() {
			defer innerWg.Done()
			buf := make([]byte, msgSize)
			for i := 0; i < msgCount; i++ {
				stream.SetReadDeadline(time.Now().Add(5 * time.Second))
				if _, err := io.ReadFull(stream, buf); err != nil {
					return
				}
			}
		}()

		innerWg.Wait()
	}()

	wg.Wait()
	t.Logf("Bidirectional data test passed: %d messages of %d bytes", msgCount, msgSize)
}
