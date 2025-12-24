package mux

import (
	"net"
)

// SessionInterface defines the common interface for multiplexing sessions
// This interface is implemented by both pkg/mux.Session (smux-based)
// and pkg/mtmux.Session (multi-tunnel multiplexer)
type SessionInterface interface {
	// OpenStream opens a new stream (client-side)
	OpenStream() (net.Conn, error)

	// AcceptStream accepts a new stream (server-side)
	AcceptStream() (net.Conn, error)

	// Close closes the session and all underlying connections
	Close() error

	// NumStreams returns the number of active streams
	NumStreams() int

	// IsClosed checks if the session is closed
	IsClosed() bool

	// RemoteAddr returns the remote address (for first connection in bundle)
	RemoteAddr() net.Addr

	// LocalAddr returns the local address (for first connection in bundle)
	LocalAddr() net.Addr
}
