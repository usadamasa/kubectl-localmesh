package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/usadamasa/kubectl-local-mesh/internal/config"
	"github.com/usadamasa/kubectl-local-mesh/internal/run"
)

func main() {
	var (
		fConfig          = flag.String("f", "", "config yaml path (e.g. services.yaml)")
		fLog             = flag.String("log", "info", "log level: debug|info|warn")
		fDumpEnvoyConfig = flag.Bool("dump-envoy-config", false, "dump envoy config to stdout and exit")
		fMockConfig      = flag.String("mock-config", "", "mock config for offline mode (works with --dump-envoy-config)")
	)
	flag.Parse()

	if *fConfig == "" && flag.NArg() >= 1 {
		*fConfig = flag.Arg(0)
	}
	if *fConfig == "" {
		fmt.Fprintln(os.Stderr, "usage: kubectl-local-mesh -f services.yaml")
		fmt.Fprintln(os.Stderr, "   or: kubectl-local-mesh services.yaml")
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

	if err := run.Run(ctx, cfg, *fLog); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
