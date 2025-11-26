package test

import (
	"bytes"
	"crypto/rand"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"qtrp/pkg/crypto"
)

// TestChaCha20Crypto 测试 ChaCha20 加密
func TestChaCha20Crypto(t *testing.T) {
	cfg := &crypto.Config{
		PSK: "test-pre-shared-key-32bytes!!!!",
	}
	crypt := crypto.NewChaCha20Crypto(cfg)

	t.Run("Type", func(t *testing.T) {
		if crypt.Type() != "chacha20" {
			t.Errorf("expected type chacha20, got %s", crypt.Type())
		}
	})

	t.Run("EncryptDecrypt_Small", func(t *testing.T) {
		testCryptoEcho(t, crypt, 1024)
	})

	t.Run("EncryptDecrypt_Medium", func(t *testing.T) {
		testCryptoEcho(t, crypt, 64*1024)
	})

	t.Run("EncryptDecrypt_Large", func(t *testing.T) {
		testCryptoEcho(t, crypt, 512*1024)
	})

	t.Run("MultipleFrames", func(t *testing.T) {
		testCryptoMultipleFrames(t, crypt)
	})
}

// TestNoneCrypto 测试无加密
func TestNoneCrypto(t *testing.T) {
	crypt := crypto.NewNoneCrypto()

	t.Run("Type", func(t *testing.T) {
		if crypt.Type() != "none" {
			t.Errorf("expected type none, got %s", crypt.Type())
		}
	})

	t.Run("Passthrough", func(t *testing.T) {
		testCryptoEcho(t, crypt, 1024)
	})
}

// testCryptoEcho 测试加密数据传输
func testCryptoEcho(t *testing.T, crypt crypto.Crypto, size int) {
	// 创建 TCP 连接对
	server, client := createConnPair(t)
	defer server.Close()
	defer client.Close()

	// 包装加密
	serverConn, err := crypt.WrapConn(server, true)
	if err != nil {
		t.Fatalf("WrapConn server failed: %v", err)
	}

	clientConn, err := crypt.WrapConn(client, false)
	if err != nil {
		t.Fatalf("WrapConn client failed: %v", err)
	}

	// 生成随机数据
	sendData := make([]byte, size)
	rand.Read(sendData)

	var wg sync.WaitGroup
	wg.Add(2)

	// 服务端读取并回显
	go func() {
		defer wg.Done()
		buf := make([]byte, size)
		n, err := io.ReadFull(serverConn, buf)
		if err != nil {
			t.Errorf("Server read failed: %v", err)
			return
		}
		if _, err := serverConn.Write(buf[:n]); err != nil {
			t.Errorf("Server write failed: %v", err)
		}
	}()

	// 客户端发送并接收
	var recvData []byte
	go func() {
		defer wg.Done()
		if _, err := clientConn.Write(sendData); err != nil {
			t.Errorf("Client write failed: %v", err)
			return
		}

		recvData = make([]byte, size)
		if _, err := io.ReadFull(clientConn, recvData); err != nil {
			t.Errorf("Client read failed: %v", err)
		}
	}()

	wg.Wait()

	// 验证数据
	if !bytes.Equal(sendData, recvData) {
		t.Error("Data mismatch after encryption/decryption")
	}

	t.Logf("Crypto echo test passed: %d bytes", size)
}

// testCryptoMultipleFrames 测试多帧传输
func testCryptoMultipleFrames(t *testing.T, crypt crypto.Crypto) {
	server, client := createConnPair(t)
	defer server.Close()
	defer client.Close()

	serverConn, _ := crypt.WrapConn(server, true)
	clientConn, _ := crypt.WrapConn(client, false)

	frameCount := 100
	frameSize := 1024

	var wg sync.WaitGroup
	wg.Add(2)

	// 服务端回显
	go func() {
		defer wg.Done()
		buf := make([]byte, frameSize)
		for i := 0; i < frameCount; i++ {
			n, err := serverConn.Read(buf)
			if err != nil {
				t.Errorf("Server read frame %d failed: %v", i, err)
				return
			}
			if _, err := serverConn.Write(buf[:n]); err != nil {
				t.Errorf("Server write frame %d failed: %v", i, err)
				return
			}
		}
	}()

	// 客户端发送接收
	go func() {
		defer wg.Done()
		sendBuf := make([]byte, frameSize)
		recvBuf := make([]byte, frameSize)

		for i := 0; i < frameCount; i++ {
			rand.Read(sendBuf)
			if _, err := clientConn.Write(sendBuf); err != nil {
				t.Errorf("Client write frame %d failed: %v", i, err)
				return
			}

			if _, err := io.ReadFull(clientConn, recvBuf); err != nil {
				t.Errorf("Client read frame %d failed: %v", i, err)
				return
			}

			if !bytes.Equal(sendBuf, recvBuf) {
				t.Errorf("Frame %d data mismatch", i)
				return
			}
		}
	}()

	wg.Wait()
	t.Logf("Multiple frames test passed: %d frames of %d bytes", frameCount, frameSize)
}

// createConnPair 创建连接对
func createConnPair(t *testing.T) (net.Conn, net.Conn) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	var server net.Conn
	var acceptErr error
	done := make(chan struct{})

	go func() {
		server, acceptErr = ln.Accept()
		close(done)
	}()

	client, err := net.DialTimeout("tcp", ln.Addr().String(), 5*time.Second)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}

	<-done
	ln.Close()

	if acceptErr != nil {
		t.Fatalf("Accept failed: %v", acceptErr)
	}

	return server, client
}
