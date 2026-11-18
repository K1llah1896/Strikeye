//go:build !windows

package ipc

import (
	"context"
	"net"
	"os"
	"path/filepath"
)

// Listen opens a local Unix domain socket listener for cross-platform testing.
func Listen(addr string) (net.Listener, error) {
	if addr == "" {
		addr = filepath.Join(os.TempDir(), "strikeye-ipc.sock")
	}
	_ = os.Remove(addr)
	if err := os.MkdirAll(filepath.Dir(addr), 0o755); err != nil {
		return nil, err
	}
	return net.Listen("unix", addr)
}

// Dial connects to a local Unix domain socket listener.
func Dial(ctx context.Context, addr string) (net.Conn, error) {
	if addr == "" {
		addr = filepath.Join(os.TempDir(), "strikeye-ipc.sock")
	}
	var d net.Dialer
	return d.DialContext(ctx, "unix", addr)
}
