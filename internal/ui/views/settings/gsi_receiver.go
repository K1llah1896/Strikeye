package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"strikeye/internal/gsi"
)

// receiverShutdownGrace bounds how long Stop waits for in-flight posts.
const receiverShutdownGrace = 2 * time.Second

// gsiReceiver hosts an in-process GSI endpoint so the library banner works
// without a terminal running `strikeye-cli gsi`.
type gsiReceiver struct {
	mu     sync.Mutex
	server *http.Server
	store  *gsi.Store
	addr   string
	mirror string
}

// start binds listen, serves the GSI handler and mirrors every update to
// cache/gsi_latest.json (the file the library banner polls).
func (r *gsiReceiver) start(listen, token, dataRoot string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.server != nil {
		return r.addr, fmt.Errorf("接收端已在运行（%s）", r.addr)
	}
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return "", fmt.Errorf("监听 %s 失败: %w", listen, err)
	}
	store := gsi.NewStore()
	mirror := gsiStatePath(dataRoot)
	if err := os.MkdirAll(filepath.Dir(mirror), 0o755); err != nil {
		ln.Close()
		return "", err
	}
	store.OnUpdate(func(st gsi.State) { _ = writeGSIStateMirror(mirror, st) })
	srv := &http.Server{Handler: store.Handler(token), ReadHeaderTimeout: 5 * time.Second}
	r.server, r.store, r.addr, r.mirror = srv, store, ln.Addr().String(), mirror
	go func() { _ = srv.Serve(ln) }()
	return r.addr, nil
}

// stop shuts the server down; it is a no-op when nothing is running.
func (r *gsiReceiver) stop() error {
	r.mu.Lock()
	srv := r.server
	addr := r.addr
	r.server, r.store, r.addr = nil, nil, ""
	r.mu.Unlock()
	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), receiverShutdownGrace)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("关闭接收端（%s）: %w", addr, err)
	}
	return nil
}

// running reports the address being served, if any.
func (r *gsiReceiver) running() (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.server == nil {
		return "", false
	}
	return r.addr, true
}

// writeGSIStateMirror atomically replaces the mirrored state file.
func writeGSIStateMirror(path string, state gsi.State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".part"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
