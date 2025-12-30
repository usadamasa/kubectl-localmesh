package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/usadamasa/kubectl-localmesh/internal/config"
	"github.com/usadamasa/kubectl-localmesh/internal/run"
)

type dumpEnvoyConfigOptions struct {
	configFile string
	mockConfig string
}

var dumpEnvoyConfigOpts = &dumpEnvoyConfigOptions{}

var dumpEnvoyConfigCmd = &cobra.Command{
	Use:   "dump-envoy-config [config-file]",
	Short: "Envoy設定をstdoutにダンプ",
	Long: `サービスを起動せずにEnvoy設定を生成してstdoutにダンプします。

以下の用途に有用です：
- 生成されるEnvoy設定の理解
- ルーティングの問題のデバッグ
- Envoy設定パターンの学習
- --mock-configによるオフライン設定検証

Examples:
  kubectl-localmesh dump-envoy-config -f services.yaml
  kubectl-localmesh dump-envoy-config services.yaml
  kubectl-localmesh dump-envoy-config -f services.yaml --mock-config mocks.yaml`,
	RunE: runDumpEnvoyConfig,
}

func init() {
	rootCmd.AddCommand(dumpEnvoyConfigCmd)

	dumpEnvoyConfigCmd.Flags().StringVarP(
		&dumpEnvoyConfigOpts.configFile,
		"config", "f", "",
		"設定ファイルのパス",
	)
	dumpEnvoyConfigCmd.Flags().StringVar(
		&dumpEnvoyConfigOpts.mockConfig,
		"mock-config", "",
		"オフラインモード用のモック設定（クラスタ接続不要）",
	)
}

func runDumpEnvoyConfig(cmd *cobra.Command, args []string) error {
	if dumpEnvoyConfigOpts.configFile == "" && len(args) > 0 {
		dumpEnvoyConfigOpts.configFile = args[0]
	}

	if dumpEnvoyConfigOpts.configFile == "" {
		return fmt.Errorf("config file required: use -f or provide as argument")
	}

	cfg, err := config.Load(dumpEnvoyConfigOpts.configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	ctx := cmd.Context()

	return run.DumpEnvoyConfig(ctx, cfg, dumpEnvoyConfigOpts.mockConfig)
}
