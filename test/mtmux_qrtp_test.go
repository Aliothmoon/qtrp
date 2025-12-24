package test

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"qtrp/internal/client"
	"qtrp/internal/server"
	"qtrp/pkg/config"
)

// TestQTRPMTMuxBasicConnection 测试 MTMux 基本连接和认证
func TestQTRPMTMuxBasicConnection(t *testing.T) {
	// 服务端配置
	serverCfg := &config.ServerConfig{
		BindAddr:      "127.0.0.1:17000",
		DashboardAddr: "",
		Transport:     "net",
		Crypto:        "none", // 简化测试，不使用加密
	}
	serverCfg.MTMux.Enabled = true
	serverCfg.MTMux.Tunnels = 4
	serverCfg.Auth.Tokens = []string{"test-token"}

	// 客户端配置
	clientCfg := &config.ClientConfig{
		ServerAddr: "127.0.0.1:17000",
		Transport:  "net",
		Crypto:     "none",
		Token:      "test-token",
	}
	clientCfg.MTMux.Enabled = true
	clientCfg.MTMux.Tunnels = 4

	// 启动服务端
	srv := server.New(serverCfg)
	go func() {
		if err := srv.Start(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()
	defer srv.Stop()

	// 等待服务器启动
	time.Sleep(500 * time.Millisecond)

	// 启动客户端
	cli, err := client.New(clientCfg)
	if err != nil {
		t.Fatalf("Create client error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- cli.Run()
	}()

	// 等待连接建立
	time.Sleep(2 * time.Second)

	// 停止客户端
	cli.Stop()

	// 检查是否有错误
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Client error: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("Test timeout")
	}

	t.Log("MTMux basic connection test passed")
}

// TestQTRPMTMuxProxyDataTransfer 测试 MTMux 代理数据传输
func TestQTRPMTMuxProxyDataTransfer(t *testing.T) {
	// 创建本地测试服务器
	localServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Create local server error: %v", err)
	}
	defer localServer.Close()
	localAddr := localServer.Addr().(*net.TCPAddr)

	// 本地服务器：回显收到的数据
	go func() {
		for {
			conn, err := localServer.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c) // 回显
			}(conn)
		}
	}()

	// 服务端配置
	serverCfg := &config.ServerConfig{
		BindAddr:      "127.0.0.1:17100",
		DashboardAddr: "",
		Transport:     "net",
		Crypto:        "none",
	}
	serverCfg.MTMux.Enabled = true
	serverCfg.MTMux.Tunnels = 8
	serverCfg.Auth.Tokens = []string{"test-token"}

	// 客户端配置
	clientCfg := &config.ClientConfig{
		ServerAddr: "127.0.0.1:17100",
		Transport:  "net",
		Crypto:     "none",
		Token:      "test-token",
		Proxies: []config.ProxyConfig{
			{
				Name:       "test-proxy",
				Type:       "tcp",
				LocalAddr:  "127.0.0.1",
				LocalPort:  fmt.Sprintf("%d", localAddr.Port),
				RemotePort: "18100",
			},
		},
	}
	clientCfg.MTMux.Enabled = true
	clientCfg.MTMux.Tunnels = 8

	// 启动服务端
	srv := server.New(serverCfg)
	go srv.Start()
	defer srv.Stop()

	time.Sleep(500 * time.Millisecond)

	// 启动客户端
	cli, err := client.New(clientCfg)
	if err != nil {
		t.Fatalf("Create client error: %v", err)
	}
	go cli.Run()
	defer cli.Stop()

	// 等待代理注册
	time.Sleep(2 * time.Second)

	// 测试数据传输
	conn, err := net.Dial("tcp", "127.0.0.1:18100")
	if err != nil {
		t.Fatalf("Connect to proxy error: %v", err)
	}
	defer conn.Close()

	testData := "Hello MTMux Proxy!"
	if _, err := conn.Write([]byte(testData)); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	buf := make([]byte, len(testData))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("Read error: %v", err)
	}

	if string(buf) != testData {
		t.Fatalf("Data mismatch: got %q, want %q", buf, testData)
	}

	t.Log("MTMux proxy data transfer test passed")
}

// TestQTRPMTMuxMultipleStreams 测试 MTMux 多流并发
func TestQTRPMTMuxMultipleStreams(t *testing.T) {
	// 创建本地测试服务器
	localServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Create local server error: %v", err)
	}
	defer localServer.Close()
	localAddr := localServer.Addr().(*net.TCPAddr)

	// 本地服务器：回显收到的数据
	go func() {
		for {
			conn, err := localServer.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(conn)
		}
	}()

	// 服务端配置
	serverCfg := &config.ServerConfig{
		BindAddr:      "127.0.0.1:17200",
		DashboardAddr: "",
		Transport:     "net",
		Crypto:        "none",
	}
	serverCfg.MTMux.Enabled = true
	serverCfg.MTMux.Tunnels = 4
	serverCfg.Auth.Tokens = []string{"test-token"}

	// 客户端配置
	clientCfg := &config.ClientConfig{
		ServerAddr: "127.0.0.1:17200",
		Transport:  "net",
		Crypto:     "none",
		Token:      "test-token",
		Proxies: []config.ProxyConfig{
			{
				Name:       "test-proxy",
				Type:       "tcp",
				LocalAddr:  "127.0.0.1",
				LocalPort:  fmt.Sprintf("%d", localAddr.Port),
				RemotePort: "18200",
			},
		},
	}
	clientCfg.MTMux.Enabled = true
	clientCfg.MTMux.Tunnels = 4

	// 启动服务端和客户端
	srv := server.New(serverCfg)
	go srv.Start()
	defer srv.Stop()

	time.Sleep(500 * time.Millisecond)

	cli, err := client.New(clientCfg)
	if err != nil {
		t.Fatalf("Create client error: %v", err)
	}
	go cli.Run()
	defer cli.Stop()

	time.Sleep(2 * time.Second)

	// 并发测试多个连接
	concurrency := 10
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			conn, err := net.Dial("tcp", "127.0.0.1:18200")
			if err != nil {
				errCh <- fmt.Errorf("connection %d dial error: %v", id, err)
				return
			}
			defer conn.Close()

			testData := fmt.Sprintf("Message from connection %d", id)
			if _, err := conn.Write([]byte(testData)); err != nil {
				errCh <- fmt.Errorf("connection %d write error: %v", id, err)
				return
			}

			buf := make([]byte, len(testData))
			if _, err := io.ReadFull(conn, buf); err != nil {
				errCh <- fmt.Errorf("connection %d read error: %v", id, err)
				return
			}

			if string(buf) != testData {
				errCh <- fmt.Errorf("connection %d data mismatch", id)
				return
			}

			errCh <- nil
		}(i)
	}

	// 等待所有连接完成
	for i := 0; i < concurrency; i++ {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}

	t.Log("MTMux multiple streams test passed")
}

// TestQTRPMTMuxVsSmuxComparison 对比测试 MTMux 和 Smux
func TestQTRPMTMuxVsSmuxComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comparison test in short mode")
	}

	testCases := []struct {
		name           string
		mtmuxEnabled   bool
		tunnels        int
		transferSizeKB int
	}{
		{"Smux", false, 0, 100},
		{"MTMux-2-Tunnels", true, 2, 100},
		{"MTMux-4-Tunnels", true, 4, 100},
		{"MTMux-8-Tunnels", true, 8, 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建本地测试服务器
			localServer, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("Create local server error: %v", err)
			}
			defer localServer.Close()
			localAddr := localServer.Addr().(*net.TCPAddr)

			// 本地服务器：丢弃所有数据（测试吞吐量）
			go func() {
				for {
					conn, err := localServer.Accept()
					if err != nil {
						return
					}
					go func(c net.Conn) {
						defer c.Close()
						io.Copy(io.Discard, c)
					}(conn)
				}
			}()

			// 服务端配置
			serverCfg := &config.ServerConfig{
				BindAddr:      fmt.Sprintf("127.0.0.1:%d", 17300+len(tc.name)),
				DashboardAddr: "",
				Transport:     "net",
				Crypto:        "none",
			}
			serverCfg.MTMux.Enabled = tc.mtmuxEnabled
			serverCfg.MTMux.Tunnels = tc.tunnels
			serverCfg.Auth.Tokens = []string{"test-token"}

			// 客户端配置
			clientCfg := &config.ClientConfig{
				ServerAddr: serverCfg.BindAddr,
				Transport:  "net",
				Crypto:     "none",
				Token:      "test-token",
				Proxies: []config.ProxyConfig{
					{
						Name:       "test-proxy",
						Type:       "tcp",
						LocalAddr:  "127.0.0.1",
						LocalPort:  fmt.Sprintf("%d", localAddr.Port),
						RemotePort: fmt.Sprintf("%d", 18300+len(tc.name)),
					},
				},
			}
			clientCfg.MTMux.Enabled = tc.mtmuxEnabled
			clientCfg.MTMux.Tunnels = tc.tunnels

			// 启动服务端和客户端
			srv := server.New(serverCfg)
			go srv.Start()
			defer srv.Stop()

			time.Sleep(500 * time.Millisecond)

			cli, err := client.New(clientCfg)
			if err != nil {
				t.Fatalf("Create client error: %v", err)
			}
			go cli.Run()
			defer cli.Stop()

			time.Sleep(2 * time.Second)

			// 测试数据传输性能
			conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", 18300+len(tc.name)))
			if err != nil {
				t.Fatalf("Connect to proxy error: %v", err)
			}
			defer conn.Close()

			data := make([]byte, tc.transferSizeKB*1024)
			start := time.Now()
			if _, err := conn.Write(data); err != nil {
				t.Fatalf("Write error: %v", err)
			}
			duration := time.Since(start)

			throughputMBps := float64(tc.transferSizeKB) / 1024.0 / duration.Seconds()
			t.Logf("%s: transferred %d KB in %v (%.2f MB/s)",
				tc.name, tc.transferSizeKB, duration, throughputMBps)
		})
	}
}
