package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"strikeye/internal/agent"
	"strikeye/internal/config"
	"strikeye/internal/updater"
	"strikeye/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "Print version information and exit")
	envCheck := flag.Bool("env-check", false, "Print runtime environment summary and exit")
	dataRootFlag := flag.String("data-root", "", "Override Strikeye DataRoot directory")
	pipeFlag := flag.String("pipe", "", "Override IPC named pipe / socket address")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Summary("strikeye-agent"))
		return
	}
	if *envCheck {
		fmt.Printf("%s: ready (phase4)\n", version.Summary("strikeye-agent"))
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

	if err := agent.Run(ctx, dataRoot, *pipeFlag); err != nil {
		fmt.Fprintf(os.Stderr, "strikeye-agent error: %v\n", err)
		os.Exit(1)
	}
}
