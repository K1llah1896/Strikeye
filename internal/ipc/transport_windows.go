//go:build windows

package ipc

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

// Listen opens a Windows Named Pipe listener at addr (defaults to DefaultPipePath).
func Listen(addr string) (net.Listener, error) {
	if addr == "" {
		addr = DefaultPipePath
	}
	return winio.ListenPipe(addr, &winio.PipeConfig{
		MessageMode:      false,
		InputBufferSize:  64 * 1024,
		OutputBufferSize: 64 * 1024,
	})
}

// Dial connects to a Windows Named Pipe server at addr (defaults to DefaultPipePath).
func Dial(ctx context.Context, addr string) (net.Conn, error) {
	if addr == "" {
		addr = DefaultPipePath
	}
	return winio.DialPipeContext(ctx, addr)
}
