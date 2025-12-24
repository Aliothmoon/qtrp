package test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/flben233/mtmux"
)

func TestMTMuxBasicConnectivity(t *testing.T) {
	tunnels := 4
	addr := "127.0.0.1:19000"

	// Start server
	ln, err := mtmux.Listen("tcp", addr, tunnels)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	// Accept connections in goroutine
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)

		bundle, err := ln.Accept()
		if err != nil {
			t.Errorf("Failed to accept: %v", err)
			return
		}

		cfg := mtmux.DefaultConfig()
		cfg.Tunnels = int32(tunnels)
		session, err := mtmux.Server(bundle, cfg)
		if err != nil {
			t.Errorf("Failed to create server session: %v", err)
			return
		}
		defer session.Close()

		ctx := context.Background()
		session.Start(ctx)                 // Non-blocking
		time.Sleep(100 * time.Millisecond) // Allow session to initialize

		// Accept a stream
		stream, err := session.AcceptStream()
		if err != nil {
			t.Errorf("Failed to accept stream: %v", err)
			return
		}
		defer stream.Close()

		// Server writes first (like the example)
		testData := []byte("Hello MTMux!")
		_, err = stream.Write(testData)
		if err != nil {
			t.Errorf("Failed to write: %v", err)
			return
		}

		// Then read response
		buf := make([]byte, 1024)
		n, err := stream.Read(buf)
		if err != nil {
			t.Errorf("Failed to read: %v", err)
			return
		}

		if string(buf[:n]) != "world" {
			t.Errorf("Expected 'world', got %s", buf[:n])
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Client connects
	bundle, err := mtmux.Dial("tcp", addr, tunnels)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	cfg := mtmux.DefaultConfig()
	cfg.Tunnels = int32(tunnels)
	session, err := mtmux.Client(bundle, cfg)
	if err != nil {
		t.Fatalf("Failed to create client session: %v", err)
	}
	defer session.Close()

	ctx := context.Background()
	session.Start(ctx)                 // Non-blocking
	time.Sleep(100 * time.Millisecond) // Allow session to initialize

	// Open a stream
	stream, err := session.OpenStream()
	if err != nil {
		t.Fatalf("Failed to open stream: %v", err)
	}
	defer stream.Close()

	// Client reads first (like the example)
	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if string(buf[:n]) != "Hello MTMux!" {
		t.Errorf("Expected 'Hello MTMux!', got %s", buf[:n])
	}

	// Then write response
	_, err = stream.Write([]byte("world"))
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	// Wait for server to finish
	select {
	case <-serverDone:
	case <-time.After(5 * time.Second):
		t.Error("Server did not finish in time")
	}
}

func TestMTMuxMultipleStreams(t *testing.T) {
	tunnels := 8
	addr := "127.0.0.1:19001"
	numStreams := 10

	// Start server
	ln, err := mtmux.Listen("tcp", addr, tunnels)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	// Accept connections in goroutine
	serverDone := make(chan struct{})
	var serverWg sync.WaitGroup
	go func() {
		defer close(serverDone)

		bundle, err := ln.Accept()
		if err != nil {
			t.Errorf("Failed to accept: %v", err)
			return
		}

		cfg := mtmux.DefaultConfig()
		cfg.Tunnels = int32(tunnels)
		session, err := mtmux.Server(bundle, cfg)
		if err != nil {
			t.Errorf("Failed to create server session: %v", err)
			return
		}
		defer session.Close()

		ctx := context.Background()
		session.Start(ctx)                 // Non-blocking
		time.Sleep(100 * time.Millisecond) // Allow session to initialize

		// Accept multiple streams
		for i := 0; i < numStreams; i++ {
			stream, err := session.AcceptStream()
			if err != nil {
				t.Errorf("Failed to accept stream %d: %v", i, err)
				return
			}

			serverWg.Add(1)
			go func(s *mtmux.Stream, idx int) {
				defer s.Close()
				defer serverWg.Done()
				// Server writes first
				testData := []byte("Hello from server")
				_, err := s.Write(testData)
				if err != nil {
					t.Errorf("Stream %d write error: %v", idx, err)
					return
				}
				// Then read response
				buf := make([]byte, 1024)
				n, err := s.Read(buf)
				if err != nil {
					t.Errorf("Stream %d read error: %v", idx, err)
					return
				}
				if string(buf[:n]) != "world" {
					t.Errorf("Stream %d: expected 'world', got %s", idx, buf[:n])
				}
			}(stream, i)
		}

		// Wait for all server streams to complete
		serverWg.Wait()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Client connects
	bundle, err := mtmux.Dial("tcp", addr, tunnels)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	cfg := mtmux.DefaultConfig()
	cfg.Tunnels = int32(tunnels)
	session, err := mtmux.Client(bundle, cfg)
	if err != nil {
		t.Fatalf("Failed to create client session: %v", err)
	}
	defer session.Close()

	ctx := context.Background()
	session.Start(ctx)                 // Non-blocking
	time.Sleep(100 * time.Millisecond) // Allow session to initialize

	// Open multiple streams
	var clientWg sync.WaitGroup
	for i := 0; i < numStreams; i++ {
		clientWg.Add(1)
		go func(idx int) {
			defer clientWg.Done()
			stream, err := session.OpenStream()
			if err != nil {
				t.Errorf("Failed to open stream %d: %v", idx, err)
				return
			}
			defer stream.Close()

			// Client reads first
			buf := make([]byte, 1024)
			n, err := stream.Read(buf)
			if err != nil {
				t.Errorf("Stream %d read error: %v", idx, err)
				return
			}

			if string(buf[:n]) != "Hello from server" {
				t.Errorf("Stream %d: expected 'Hello from server', got %s", idx, buf[:n])
				return
			}

			// Then write response
			_, err = stream.Write([]byte("world"))
			if err != nil {
				t.Errorf("Stream %d write error: %v", idx, err)
				return
			}
		}(i)
	}

	// Wait for all client streams to complete
	clientWg.Wait()

	// Wait for server to finish
	select {
	case <-serverDone:
	case <-time.After(5 * time.Second):
		t.Error("Server did not finish in time")
	}
}

func TestMTMuxVaryingTunnelCounts(t *testing.T) {
	testCases := []int{1, 2, 4, 8}

	for _, tunnels := range testCases {
		t.Run(string(rune('0'+tunnels))+" tunnels", func(t *testing.T) {
			addr := "127.0.0.1:19002"

			ln, err := mtmux.Listen("tcp", addr, tunnels)
			if err != nil {
				t.Fatalf("Failed to listen with %d tunnels: %v", tunnels, err)
			}
			defer ln.Close()

			serverDone := make(chan struct{})
			go func() {
				defer close(serverDone)

				bundle, err := ln.Accept()
				if err != nil {
					t.Errorf("Failed to accept: %v", err)
					return
				}

				cfg := mtmux.DefaultConfig()
				cfg.Tunnels = int32(tunnels)
				session, err := mtmux.Server(bundle, cfg)
				if err != nil {
					t.Errorf("Failed to create server session: %v", err)
					return
				}
				defer session.Close()

				ctx := context.Background()
				session.Start(ctx)                 // Non-blocking
				time.Sleep(100 * time.Millisecond) // Allow session to initialize

				stream, err := session.AcceptStream()
				if err != nil {
					t.Errorf("Failed to accept stream: %v", err)
					return
				}
				defer stream.Close()

				// Server writes first
				testData := []byte("Test with tunnels")
				_, err = stream.Write(testData)
				if err != nil {
					t.Errorf("Failed to write: %v", err)
					return
				}
				// Then read response
				buf := make([]byte, 1024)
				n, err := stream.Read(buf)
				if err != nil {
					t.Errorf("Failed to read: %v", err)
					return
				}
				if string(buf[:n]) != "world" {
					t.Errorf("Expected 'world', got %s", buf[:n])
				}
			}()

			time.Sleep(100 * time.Millisecond)

			bundle, err := mtmux.Dial("tcp", addr, tunnels)
			if err != nil {
				t.Fatalf("Failed to dial: %v", err)
			}

			cfg := mtmux.DefaultConfig()
			cfg.Tunnels = int32(tunnels)
			session, err := mtmux.Client(bundle, cfg)
			if err != nil {
				t.Fatalf("Failed to create client session: %v", err)
			}
			defer session.Close()

			ctx := context.Background()
			session.Start(ctx)                 // Non-blocking
			time.Sleep(100 * time.Millisecond) // Allow session to initialize

			stream, err := session.OpenStream()
			if err != nil {
				t.Fatalf("Failed to open stream: %v", err)
			}
			defer stream.Close()

			// Client reads first
			buf := make([]byte, 1024)
			n, err := stream.Read(buf)
			if err != nil {
				t.Fatalf("Failed to read: %v", err)
			}

			if string(buf[:n]) != "Test with tunnels" {
				t.Errorf("Expected 'Test with tunnels', got %s", buf[:n])
			}

			// Then write response
			_, err = stream.Write([]byte("world"))
			if err != nil {
				t.Fatalf("Failed to write: %v", err)
			}

			select {
			case <-serverDone:
			case <-time.After(5 * time.Second):
				t.Error("Server did not finish in time")
			}
		})
	}
}
