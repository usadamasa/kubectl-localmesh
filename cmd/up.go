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
	noEditHosts bool
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
  kubectl-localmesh up -f services.yaml --no-edit-hosts`,
	RunE: runUp,
}

func init() {
	rootCmd.AddCommand(upCmd)

	upCmd.Flags().StringVarP(&upOpts.configFile, "config", "f", "", "config yaml path")
	upCmd.Flags().BoolVar(&upOpts.noEditHosts, "no-edit-hosts", false, "skip updating /etc/hosts")
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

	// シグナルハンドリング
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 論理反転: noEditHosts=false → updateHosts=true
	updateHosts := !upOpts.noEditHosts

	return run.Run(ctx, cfg, globalLogLevel, updateHosts)
}
