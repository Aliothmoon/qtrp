//go:build windows

package pool

import (
	"syscall"

	"golang.org/x/sys/windows"
)

func SetReuseAddr(network, address string, c syscall.RawConn) error {
	var sockErr error
	err := c.Control(func(fd uintptr) {
		// Windows 上设置 SO_REUSEADDR
		sockErr = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1)
	})
	if err != nil {
		return err
	}
	return sockErr
}
