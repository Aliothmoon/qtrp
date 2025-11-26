package test

import (
	"crypto/rand"
	"io"
	"net"
	"sync"
	"testing"

	"qtrp/pkg/crypto"
	"qtrp/pkg/mux"
	"qtrp/pkg/transport"
)

// BenchmarkNetTransportThroughput 测试 net 传输吞吐量
func BenchmarkNetTransportThroughput(b *testing.B) {
	trans := transport.NewNetTransport()
	benchmarkTransportThroughput(b, trans)
}

// BenchmarkQUICTransportThroughput 测试 QUIC 传输吞吐量（内置加密+多路复用）
func BenchmarkQUICTransportThroughput(b *testing.B) {
	trans := transport.NewQUICTransport()
	benchmarkTransportThroughput(b, trans)
}

// BenchmarkChaCha20Throughput 测试 ChaCha20 加密吞吐量
func BenchmarkChaCha20Throughput(b *testing.B) {
	cfg := &crypto.Config{PSK: "benchmark-key-32-bytes-long!!!!"}
	crypt := crypto.NewChaCha20Crypto(cfg)

	server, client := createBenchConnPair(b)
	defer server.Close()
	defer client.Close()

	serverConn, _ := crypt.WrapConn(server, true)
	clientConn, _ := crypt.WrapConn(client, false)

	data := make([]byte, 64*1024) // 64KB
	rand.Read(data)

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		buf := make([]byte, len(data))
		for i := 0; i < b.N; i++ {
			io.ReadFull(serverConn, buf)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < b.N; i++ {
			clientConn.Write(data)
		}
	}()

	wg.Wait()
}

// BenchmarkMuxThroughput 测试 yamux 吞吐量
func BenchmarkMuxThroughput(b *testing.B) {
	server, client := createBenchConnPair(b)

	serverSession, _ := mux.NewServerSession(server)
	defer serverSession.Close()
	clientSession, _ := mux.NewClientSession(client)
	defer clientSession.Close()

	data := make([]byte, 64*1024)
	rand.Read(data)

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		stream, _ := serverSession.AcceptStream()
		defer stream.Close()
		buf := make([]byte, len(data))
		for i := 0; i < b.N; i++ {
			io.ReadFull(stream, buf)
		}
	}()

	go func() {
		defer wg.Done()
		stream, _ := clientSession.OpenStream()
		defer stream.Close()
		for i := 0; i < b.N; i++ {
			stream.Write(data)
		}
	}()

	wg.Wait()
}

// BenchmarkFullStackChaCha20 测试完整协议栈吞吐量（ChaCha20）
func BenchmarkFullStackChaCha20(b *testing.B) {
	server, client := createBenchConnPair(b)

	cfg := &crypto.Config{PSK: "benchmark-key-32-bytes-long!!!!"}
	crypt := crypto.NewChaCha20Crypto(cfg)
	serverCrypt, _ := crypt.WrapConn(server, true)
	clientCrypt, _ := crypt.WrapConn(client, false)

	serverSession, _ := mux.NewServerSession(serverCrypt)
	defer serverSession.Close()
	clientSession, _ := mux.NewClientSession(clientCrypt)
	defer clientSession.Close()

	data := make([]byte, 64*1024) // 64KB
	rand.Read(data)

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		stream, _ := serverSession.AcceptStream()
		defer stream.Close()
		buf := make([]byte, len(data))
		for i := 0; i < b.N; i++ {
			io.ReadFull(stream, buf)
		}
	}()

	go func() {
		defer wg.Done()
		stream, _ := clientSession.OpenStream()
		defer stream.Close()
		for i := 0; i < b.N; i++ {
			stream.Write(data)
		}
	}()

	wg.Wait()
}

// BenchmarkFullStackNone 测试完整协议栈吞吐量（无加密）
func BenchmarkFullStackNone(b *testing.B) {
	server, client := createBenchConnPair(b)

	serverSession, _ := mux.NewServerSession(server)
	defer serverSession.Close()
	clientSession, _ := mux.NewClientSession(client)
	defer clientSession.Close()

	data := make([]byte, 64*1024) // 64KB
	rand.Read(data)

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		stream, _ := serverSession.AcceptStream()
		defer stream.Close()
		buf := make([]byte, len(data))
		for i := 0; i < b.N; i++ {
			io.ReadFull(stream, buf)
		}
	}()

	go func() {
		defer wg.Done()
		stream, _ := clientSession.OpenStream()
		defer stream.Close()
		for i := 0; i < b.N; i++ {
			stream.Write(data)
		}
	}()

	wg.Wait()
}

func benchmarkTransportThroughput(b *testing.B, trans transport.Transport) {
	ln, err := trans.Listen("127.0.0.1:0")
	if err != nil {
		b.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	data := make([]byte, 64*1024) // 64KB
	rand.Read(data)

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		conn, _ := ln.Accept()
		defer conn.Close()
		buf := make([]byte, len(data))
		for i := 0; i < b.N; i++ {
			io.ReadFull(conn, buf)
		}
	}()

	go func() {
		defer wg.Done()
		conn, _ := trans.Dial(addr)
		defer conn.Close()
		for i := 0; i < b.N; i++ {
			conn.Write(data)
		}
	}()

	wg.Wait()
}

func createBenchConnPair(b *testing.B) (net.Conn, net.Conn) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("Listen failed: %v", err)
	}

	var server net.Conn
	done := make(chan struct{})

	go func() {
		server, _ = ln.Accept()
		close(done)
	}()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		b.Fatalf("Dial failed: %v", err)
	}

	<-done
	ln.Close()

	return server, client
}
