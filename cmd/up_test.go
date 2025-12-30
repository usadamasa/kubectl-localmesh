package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestUpCommand_FlagParsing(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantConfig  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "flag形式で設定ファイル指定",
			args:       []string{"-f", "testdata/test-services.yaml"},
			wantConfig: "testdata/test-services.yaml",
			wantErr:    false,
		},
		{
			name:       "long flag形式で設定ファイル指定",
			args:       []string{"--config", "testdata/test-services.yaml"},
			wantConfig: "testdata/test-services.yaml",
			wantErr:    false,
		},
		{
			name:       "位置引数で設定ファイル指定",
			args:       []string{"testdata/test-services.yaml"},
			wantConfig: "testdata/test-services.yaml",
			wantErr:    false,
		},
		{
			name:        "設定ファイル未指定",
			args:        []string{},
			wantErr:     true,
			errContains: "config file required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// upOptsをリセット
			upOpts = &upOptions{}

			cmd := &cobra.Command{
				Use: "test",
				RunE: func(cmd *cobra.Command, args []string) error {
					// フラグが指定されていない場合、位置引数を使用
					if upOpts.configFile == "" && len(args) > 0 {
						upOpts.configFile = args[0]
					}

					if upOpts.configFile == "" {
						return fmt.Errorf("config file required")
					}
					return nil
				},
			}

			cmd.Flags().StringVarP(&upOpts.configFile, "config", "f", "", "config yaml path")
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Error message should contain %q, got %q", tt.errContains, err.Error())
				}
			}

			if !tt.wantErr && upOpts.configFile != tt.wantConfig {
				t.Errorf("configFile = %v, want %v", upOpts.configFile, tt.wantConfig)
			}
		})
	}
}

func TestUpCommand_DumpEnvoyConfig(t *testing.T) {
	// upOptsをリセット
	upOpts = &upOptions{}

	cmd := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.Flags().StringVarP(&upOpts.configFile, "config", "f", "", "config yaml path")
	cmd.Flags().BoolVar(&upOpts.dumpConfig, "dump-envoy-config", false, "dump envoy config")
	cmd.SetArgs([]string{"-f", "testdata/test-services.yaml", "--dump-envoy-config"})

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}

	if !upOpts.dumpConfig {
		t.Errorf("dumpConfig should be true")
	}
}

func TestUpCommand_LogLevel(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantLogLevel string
	}{
		{
			name:         "デフォルトログレベル",
			args:         []string{"-f", "testdata/test-services.yaml"},
			wantLogLevel: "info",
		},
		{
			name:         "debugログレベル",
			args:         []string{"-f", "testdata/test-services.yaml", "--log-level", "debug"},
			wantLogLevel: "debug",
		},
		{
			name:         "warnログレベル",
			args:         []string{"-f", "testdata/test-services.yaml", "--log-level", "warn"},
			wantLogLevel: "warn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// upOptsをリセット
			upOpts = &upOptions{
				logLevel: "info", // デフォルト値
			}

			cmd := &cobra.Command{
				Use: "test",
				RunE: func(cmd *cobra.Command, args []string) error {
					if upOpts.configFile == "" && len(args) > 0 {
						upOpts.configFile = args[0]
					}
					return nil
				},
			}

			cmd.Flags().StringVarP(&upOpts.configFile, "config", "f", "", "config yaml path")
			cmd.Flags().StringVar(&upOpts.logLevel, "log-level", "info", "log level")
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err != nil {
				t.Errorf("Execute() error = %v", err)
			}

			if upOpts.logLevel != tt.wantLogLevel {
				t.Errorf("logLevel = %v, want %v", upOpts.logLevel, tt.wantLogLevel)
			}
		})
	}
}

func TestUpCommand_UpdateHosts(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		wantUpdateHosts bool
	}{
		{
			name:            "デフォルト（true）",
			args:            []string{"-f", "testdata/test-services.yaml"},
			wantUpdateHosts: true,
		},
		{
			name:            "明示的にfalse",
			args:            []string{"-f", "testdata/test-services.yaml", "--update-hosts=false"},
			wantUpdateHosts: false,
		},
		{
			name:            "明示的にtrue",
			args:            []string{"-f", "testdata/test-services.yaml", "--update-hosts=true"},
			wantUpdateHosts: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// upOptsをリセット
			upOpts = &upOptions{
				updateHosts: true, // デフォルト値
			}

			cmd := &cobra.Command{
				Use: "test",
				RunE: func(cmd *cobra.Command, args []string) error {
					if upOpts.configFile == "" && len(args) > 0 {
						upOpts.configFile = args[0]
					}
					return nil
				},
			}

			cmd.Flags().StringVarP(&upOpts.configFile, "config", "f", "", "config yaml path")
			cmd.Flags().BoolVar(&upOpts.updateHosts, "update-hosts", true, "update /etc/hosts")
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err != nil {
				t.Errorf("Execute() error = %v", err)
			}

			if upOpts.updateHosts != tt.wantUpdateHosts {
				t.Errorf("updateHosts = %v, want %v", upOpts.updateHosts, tt.wantUpdateHosts)
			}
		})
	}
}

func TestMain(m *testing.M) {
	// テスト実行
	code := m.Run()
	os.Exit(code)
}
