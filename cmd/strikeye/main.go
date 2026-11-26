package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"strikeye/internal/agent"
	"strikeye/internal/cli"
	"strikeye/internal/config"
	"strikeye/internal/ipc"
	"strikeye/internal/storage"
	"strikeye/internal/ui"
	"strikeye/internal/ui/guard"
	"strikeye/internal/updater"
	"strikeye/internal/version"
)

func main() {
	mode, args := resolveMode(os.Args[0], os.Args[1:])
	switch mode {
	case modeCLI:
		attachConsoleForCLI()
		cli.Run(args)
		return
	case modeAgent:
		if err := runAgentEntry(args); err != nil {
			fmt.Fprintf(os.Stderr, "strikeye-agent error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	detachConsoleForDesktop()

	showVersion := flag.Bool("version", false, "Print version information and exit")
	envCheck := flag.Bool("env-check", false, "Print runtime environment summary and exit")
	agentMode := flag.Bool("agent", false, "Run headless background agent service")
	dataRootFlag := flag.String("data-root", "", "Override Strikeye DataRoot directory")
	pipeFlag := flag.String("pipe", "", "Override IPC named pipe / socket address")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Summary("strikeye"))
		return
	}
	if *envCheck {
		fmt.Printf("%s: ready (phase5 native ui)\n", version.Summary("strikeye"))
		return
	}
	if exe, err := os.Executable(); err == nil {
		updater.CleanupOldBinaries(filepath.Dir(exe))
	}

	dataRoot := *dataRootFlag
	if dataRoot == "" {
		var err error
		dataRoot, err = config.DefaultDataRoot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve data-root: %v\n", err)
			os.Exit(1)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	defer func() {
		if r := recover(); r != nil {
			logStartupError(dataRoot, fmt.Sprintf("panic: %v\n%s", r, string(debug.Stack())))
			os.Exit(1)
		}
	}()

	if *agentMode {
		if err := agent.Run(ctx, dataRoot, *pipeFlag); err != nil {
			logStartupError(dataRoot, fmt.Sprintf("strikeye-agent error: %v", err))
			fmt.Fprintf(os.Stderr, "strikeye-agent error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := runDesktop(ctx, dataRoot, *pipeFlag); err != nil {
		logStartupError(dataRoot, fmt.Sprintf("strikeye error: %v", err))
		fmt.Fprintf(os.Stderr, "strikeye error: %v\n", err)
		os.Exit(1)
	}
}

func logStartupError(dataRoot, msg string) {
	if dataRoot == "" {
		return
	}
	f, err := os.OpenFile(filepath.Join(dataRoot, "strikeye-startup.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), msg)
	_ = f.Close()
}

func resolvePipeAddr(dataRoot, override string) string {
	if override != "" {
		return override
	}
	if os.PathSeparator == '\\' {
		return ipc.DefaultPipePath
	}
	return filepath.Join(dataRoot, "strikeye-ipc.sock")
}

func connectOrSpawnAgent(ctx context.Context, dataRoot, pipeAddr string) *ipc.Client {
	dialOnce := func() *ipc.Client {
		dctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
		defer cancel()
		conn, err := ipc.Dial(dctx, pipeAddr)
		if err != nil {
			return nil
		}
		cli := ipc.NewClient(conn)
		var ping map[string]any
		if err := cli.Call(dctx, "agent.ping", nil, &ping); err != nil {
			_ = cli.Close()
			return nil
		}
		return cli
	}

	if cli := dialOnce(); cli != nil {
		return cli
	}

	// Auto-spawn integrated background agent via self-invocation (--agent).
	selfExe, err := os.Executable()
	var cmd *exec.Cmd
	if err == nil && selfExe != "" {
		cmd = exec.Command(selfExe, "--agent", "--data-root", dataRoot, "--pipe", pipeAddr)
	} else {
		agentBin := "strikeye-agent"
		if os.PathSeparator == '\\' {
			agentBin = "strikeye-agent.exe"
		}
		cmd = exec.Command(agentBin, "--data-root", dataRoot, "--pipe", pipeAddr)
	}
	detachAgentProcess(cmd)
	_ = cmd.Start()

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if cli := dialOnce(); cli != nil {
			return cli
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(200 * time.Millisecond):
		}
	}
	return nil
}

func shouldShowGuardDialog(renderLock ipc.Lock) (bool, error) {
	if renderLock == nil {
		return false, nil
	}
	return renderLock.IsHeld()
}

func runDesktop(ctx context.Context, dataRoot, pipeOverride string) error {
	lockDir := filepath.Join(dataRoot, "cache")
	pipeAddr := resolvePipeAddr(dataRoot, pipeOverride)

	renderLock := ipc.NewLockInDir(lockDir, ipc.MutexCS2RenderActive)
	if held, err := shouldShowGuardDialog(renderLock); err == nil && held {
		agentCli := connectOrSpawnAgent(ctx, dataRoot, pipeAddr)
		if agentCli != nil {
			defer agentCli.Close()
		}
		dec, err := guard.NewController(agentCli, renderLock).RunModal(ctx)
		if err != nil {
			return err
		}
		if dec != guard.DecisionOpenUI {
			return nil
		}
	}

	uiLock := ipc.NewLockInDir(lockDir, ipc.MutexUISingleton)
	acquired, err := uiLock.TryAcquire()
	if err != nil {
		return fmt.Errorf("acquire UI singleton lock: %w", err)
	}
	if !acquired {
		return nil
	}
	defer func() { _ = uiLock.Release() }()

	agentCli := connectOrSpawnAgent(ctx, dataRoot, pipeAddr)
	if agentCli != nil {
		defer agentCli.Close()
	}

	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()

	app := ui.NewApp(ui.AppOptions{
		DataRoot: dataRoot,
		DB:       db,
		Agent:    agentCli,
	})
	err = app.Run(ctx)
	if agentCli != nil && !app.ShouldKeepAgentOnExit() {
		sctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		_ = agentCli.Call(sctx, "agent.shutdown", nil, nil)
		cancel()
	}
	return err
}

// runAgentEntry runs the background agent in this process for the `agent`
// selector and for a binary renamed to `strikeye-agent`. The flags match the
// stand-alone agent so a self-spawned child keeps working either way.
func runAgentEntry(args []string) error {
	fs := flag.NewFlagSet("strikeye-agent", flag.ExitOnError)
	showVersion := fs.Bool("version", false, "Print version information and exit")
	envCheck := fs.Bool("env-check", false, "Print runtime environment summary and exit")
	dataRoot := fs.String("data-root", "", "Override Strikeye DataRoot directory")
	pipe := fs.String("pipe", "", "Override IPC named pipe / socket address")
	_ = fs.Parse(args)

	if *showVersion {
		fmt.Println(version.Summary("strikeye-agent"))
		return nil
	}
	if *envCheck {
		fmt.Printf("%s: ready (phase4)\n", version.Summary("strikeye-agent"))
		return nil
	}
	root := *dataRoot
	if root == "" {
		resolved, err := config.DefaultDataRoot()
		if err != nil {
			return fmt.Errorf("resolve data-root: %w", err)
		}
		root = resolved
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return agent.Run(ctx, root, *pipe)
}
