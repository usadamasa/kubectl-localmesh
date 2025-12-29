package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/usadamasa/kubectl-localmesh/internal/config"
	"github.com/usadamasa/kubectl-localmesh/internal/run"
)

func main() {
	var (
		fConfig          = flag.String("f", "", "config yaml path (e.g. services.yaml)")
		fLog             = flag.String("log", "info", "log level: debug|info|warn")
		fDumpEnvoyConfig = flag.Bool("dump-envoy-config", false, "dump envoy config to stdout and exit")
		fMockConfig      = flag.String("mock-config", "", "mock config for offline mode (works with --dump-envoy-config)")
		fUpdateHosts     = flag.Bool("update-hosts", true, "update /etc/hosts (requires sudo)")
	)
	flag.Parse()

	if *fConfig == "" && flag.NArg() >= 1 {
		*fConfig = flag.Arg(0)
	}
	if *fConfig == "" {
		fmt.Fprintln(os.Stderr, "usage: kubectl-localmesh -f services.yaml")
		fmt.Fprintln(os.Stderr, "   or: kubectl-localmesh services.yaml")
		os.Exit(2)
	}

	cfg, err := config.Load(*fConfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if *fDumpEnvoyConfig {
		if err := run.DumpEnvoyConfig(ctx, cfg, *fMockConfig); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	}

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	if err := run.Run(ctx, cfg, *fLog, *fUpdateHosts); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
