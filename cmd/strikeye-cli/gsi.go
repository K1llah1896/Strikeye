package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"strikeye/internal/gsi"
)

// gsiOptions holds the flags of the `gsi` subcommand.
type gsiOptions struct {
	listen     string
	token      string
	configOut  string
	latestPath string
	once       bool
	timeout    time.Duration
}

// runGSI serves the GSI endpoint, writes the integration config and mirrors the
// latest state to disk so the UI (or a script) can poll it.
func runGSI(ctx context.Context, dataRoot string, opts gsiOptions, out io.Writer) error {
	if opts.listen == "" {
		opts.listen = "127.0.0.1:32123"
	}
	if opts.latestPath == "" {
		opts.latestPath = filepath.Join(dataRoot, "cache", "gsi_latest.json")
	}
	if opts.configOut == "" {
		opts.configOut = filepath.Join(dataRoot, "cache", "gsi", "gamestate_integration_strikeye.cfg")
	}

	store := gsi.NewStore()
	if err := os.MkdirAll(filepath.Dir(opts.configOut), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(opts.latestPath), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	uri := "http://" + normalizeListen(opts.listen) + "/"
	if err := os.WriteFile(opts.configOut, []byte(gsi.Config(uri, opts.token)), 0o644); err != nil {
		return fmt.Errorf("write gsi config: %w", err)
	}
	fmt.Fprintf(out, "gsi config: %s -> %s\n", opts.configOut, uri)

	ln, err := net.Listen("tcp", opts.listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", opts.listen, err)
	}
	srv := &http.Server{Handler: store.Handler(opts.token), ReadHeaderTimeout: 5 * time.Second}
	defer srv.Close()

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		close(done)
	}()

	first := make(chan struct{}, 1)
	store.OnUpdate(func(st gsi.State) {
		if err := writeLatestState(opts.latestPath, st); err != nil {
			fmt.Fprintf(os.Stderr, "gsi state write: %v\n", err)
		}
		line, _ := json.Marshal(gsiSummary(st))
		fmt.Fprintln(out, string(line))
		select {
		case first <- struct{}{}:
		default:
		}
	})

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	fmt.Fprintf(out, "listening on %s (post payloads to %s)\n", opts.listen, uri)

	if opts.once {
		if opts.timeout <= 0 {
			opts.timeout = 30 * time.Second
		}
		select {
		case <-first:
		case <-time.After(opts.timeout):
			return fmt.Errorf("no gsi payload within %s", opts.timeout)
		case <-ctx.Done():
			return ctx.Err()
		}
		st, _ := store.Latest()
		return writeLatestState(opts.latestPath, st)
	}

	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	case <-done:
		return nil
	}
}

// normalizeListen turns ":32123" / "0.0.0.0:32123" into a dialable host.
func normalizeListen(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return listen
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

// gsiSummary is the one-line digest printed for each payload.
func gsiSummary(st gsi.State) map[string]any {
	aliveCT, aliveT := 0, 0
	for _, p := range st.Players {
		if !p.Alive {
			continue
		}
		if p.Side == "CT" {
			aliveCT++
		} else if p.Side == "T" {
			aliveT++
		}
	}
	return map[string]any{
		"map":             st.Map,
		"phase":           st.Phase,
		"round":           st.Round,
		"score":           fmt.Sprintf("%d-%d", st.TeamScoreCT, st.TeamScoreT),
		"round_phase":     st.RoundPhase,
		"bomb":            st.Bomb,
		"players":         len(st.Players),
		"alive_ct":        aliveCT,
		"alive_t":         aliveT,
		"round_time_left": st.RoundTimeLeft,
	}
}

// writeLatestState atomically mirrors the last payload to disk.
func writeLatestState(path string, st gsi.State) error {
	body, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".part"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// signalContext cancels when the process receives SIGINT/SIGTERM.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
