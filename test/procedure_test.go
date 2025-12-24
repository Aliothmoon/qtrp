package test

import (
	"fmt"
	"net"
	"qtrp/internal/client"
	"qtrp/internal/server"
	"qtrp/pkg/config"
	"strconv"
	"testing"
	"time"
)

func startRemoteClient(t *testing.T) {
	conn, err := net.Dial("tcp", "127.0.0.1:12366")
	if err != nil {
		t.Errorf("Client dial error: %v", err)
		fmt.Println(err)
		return
	}
	defer conn.Close()
	for i := 0; i < 10; i++ {
		conn.Write([]byte("Hello 1 " + strconv.Itoa(i)))
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			t.Errorf("Client read error: %v", err)
		}
		if string(buf[:n]) != "Hello 2 "+strconv.Itoa(i) {
			t.Errorf("Client expected 'Hello 2', got %s", buf[:n])
		}
		fmt.Println("Remote client received:", string(buf[:n]))
	}
	time.Sleep(1000 * time.Millisecond)
}

func startLocalServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:16002")
	if err != nil {
		t.Errorf("Server listen error: %v", err)
		fmt.Println(err)
	}
	defer ln.Close()

	conn, err := ln.Accept()
	if err != nil {
		t.Errorf("Server accept error: %v", err)
	}
	defer conn.Close()

	for i := 0; i < 10; i++ {
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			t.Errorf("Server read error: %v", err)
		}
		if string(buf[:n]) != "Hello 1 "+strconv.Itoa(i) {
			t.Errorf("Server expected 'Hello 1', got %s", buf[:n])
		}
		conn.Write([]byte("Hello 2 " + strconv.Itoa(i)))
		fmt.Println("Local server received:", string(buf[:n]))
	}
	time.Sleep(1000 * time.Millisecond)
}

func startQTRPServer(t *testing.T) chan interface{} {
	cfg, err := config.LoadServerConfig("server.yaml")
	if err != nil {
		t.Fatalf("[server] load config error: %v", err)
	}

	t.Logf("[server] config loaded: bind_addr=%s, mtmux.enabled=%v, mtmux.tunnels=%d",
		cfg.BindAddr, cfg.MTMux.Enabled, cfg.MTMux.Tunnels)

	srv := server.New(cfg)

	// 优雅退出
	sigCh := make(chan interface{}, 1)
	go func() {
		sig := <-sigCh
		t.Logf("[server] received signal: %v, shutting down...", sig)
		if err := srv.Stop(); err != nil {
			t.Logf("[server] stop error: %v", err)
		}
	}()

	go func() {
		t.Log("[server] starting server")
		if err := srv.Start(); err != nil {
			t.Logf("[server] server error: %v", err)
		}
	}()

	return sigCh
}

func startQTRPClient(t *testing.T) chan interface{} {
	cfg, err := config.LoadClientConfig("client.yaml")
	if err != nil {
		t.Fatalf("[client] load config error: %v", err)
	}

	t.Logf("[client] config loaded: server_addr=%s, mtmux.enabled=%v, mtmux.tunnels=%d",
		cfg.ServerAddr, cfg.MTMux.Enabled, cfg.MTMux.Tunnels)

	cli, err := client.New(cfg)
	if err != nil {
		t.Fatalf("[client] create client error: %v", err)
	}

	// 优雅退出
	sigCh := make(chan interface{}, 1)
	go func() {
		sig := <-sigCh
		t.Logf("[client] received signal: %v, shutting down...", sig)
		cli.Stop()
		t.Log("[client] stopped gracefully")
	}()

	go func() {
		t.Log("[client] starting client")
		if err := cli.Run(); err != nil {
			t.Logf("[client] client error: %v", err)
		}
	}()

	return sigCh
}

func TestQTRPWithMTMux(t *testing.T) {
	// Start components in the correct order:
	// 1. Local server (the actual service we're proxying)
	// 2. QTRP server (will listen on 7000-7007 with MTMux)
	// 3. QTRP client (will connect and register proxy)
	// 4. Remote client (connects to the exposed port)

	t.Log("Starting local server on :16002")
	go startLocalServer(t)
	time.Sleep(500 * time.Millisecond)

	t.Log("Starting QTRP server on :7000 (with MTMux tunnels 7000-7007)")
	serverSigCh := startQTRPServer(t)
	defer func() { serverSigCh <- "test_done" }()
	time.Sleep(1000 * time.Millisecond)

	t.Log("Starting QTRP client")
	clientSigCh := startQTRPClient(t)
	defer func() { clientSigCh <- "test_done" }()

	// Give more time for MTMux connection establishment and proxy registration
	// MTMux needs to:
	// 1. Establish 8 parallel connections (1-2s)
	// 2. Complete authentication (0.5s)
	// 3. Register proxy (0.5s)
	t.Log("Waiting for MTMux connection and proxy registration...")
	time.Sleep(5000 * time.Millisecond)

	t.Log("Starting remote client to connect to :12366")
	go startRemoteClient(t)

	// Wait for test to complete
	time.Sleep(5000 * time.Millisecond)
	t.Log("Test completed successfully")
}
