package mux

import (
	"net"

	"github.com/flben233/mtmux"
)

// SessionAdapter adapts mtmux.Session to implement mux.SessionInterface
// This allows mtmux sessions to be used interchangeably with smux sessions
type SessionAdapter struct {
	*mtmux.Session
}

// NewSessionAdapter wraps a mtmux.Session to implement mux.SessionInterface
func NewSessionAdapter(session *mtmux.Session) *SessionAdapter {
	return &SessionAdapter{Session: session}
}

// OpenStream opens a new stream (client-side)
// Returns net.Conn to match the interface
func (a *SessionAdapter) OpenStream() (net.Conn, error) {
	stream, err := a.Session.OpenStream()
	if err != nil {
		return nil, err
	}
	return stream, nil
}

// AcceptStream accepts a new stream (server-side)
// Returns net.Conn to match the interface
func (a *SessionAdapter) AcceptStream() (net.Conn, error) {
	stream, err := a.Session.AcceptStream()
	if err != nil {
		return nil, err
	}
	return stream, nil
}

// Close closes the session and all underlying connections
func (a *SessionAdapter) Close() error {
	return a.Session.Close()
}

// NumStreams returns the number of active streams
func (a *SessionAdapter) NumStreams() int {
	return a.Session.NumStreams()
}

// IsClosed checks if the session is closed
func (a *SessionAdapter) IsClosed() bool {
	return a.Session.IsClosed()
}

// RemoteAddr returns the remote address of the first connection
func (a *SessionAdapter) RemoteAddr() net.Addr {
	return a.Session.RemoteAddr
}

// LocalAddr returns the local address of the first connection
func (a *SessionAdapter) LocalAddr() net.Addr {
	return a.Session.LocalAddr
}
