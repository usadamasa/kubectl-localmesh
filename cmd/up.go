package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/usadamasa/kubectl-localmesh/internal/config"
	"github.com/usadamasa/kubectl-localmesh/internal/run"
)

type upOptions struct {
	configFile  string
	logLevel    string
	dumpConfig  bool
	mockConfig  string
	updateHosts bool
}

var upOpts = &upOptions{}

var upCmd = &cobra.Command{
	Use:   "up [config-file]",
	Short: "Start the local service mesh",
	Long: `Start kubectl port-forward processes for all configured services
and run a local Envoy proxy for host-based routing.

Examples:
  kubectl-localmesh up -f services.yaml
  kubectl-localmesh up services.yaml
  kubectl-localmesh up -f services.yaml --dump-envoy-config`,
	RunE: runUp,
}

func init() {
	rootCmd.AddCommand(upCmd)

	upCmd.Flags().StringVarP(&upOpts.configFile, "config", "f", "", "config yaml path")
	upCmd.Flags().StringVar(&upOpts.logLevel, "log-level", "info", "log level: debug|info|warn")
	upCmd.Flags().BoolVar(&upOpts.dumpConfig, "dump-envoy-config", false, "dump envoy config to stdout and exit")
	upCmd.Flags().StringVar(&upOpts.mockConfig, "mock-config", "", "mock config for offline mode (works with --dump-envoy-config)")
	upCmd.Flags().BoolVar(&upOpts.updateHosts, "update-hosts", true, "update /etc/hosts (requires sudo)")
}

func runUp(cmd *cobra.Command, args []string) error {
	// フラグが指定されていない場合、位置引数を使用
	if upOpts.configFile == "" && len(args) > 0 {
		upOpts.configFile = args[0]
	}

	if upOpts.configFile == "" {
		return fmt.Errorf("config file required: use -f or provide as argument")
	}

	// 設定ファイルの読み込み
	cfg, err := config.Load(upOpts.configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	ctx := cmd.Context()

	// --dump-envoy-configモード
	if upOpts.dumpConfig {
		return run.DumpEnvoyConfig(ctx, cfg, upOpts.mockConfig)
	}

	// メインモード: シグナルハンドリング + run.Run()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	return run.Run(ctx, cfg, upOpts.logLevel, upOpts.updateHosts)
}
