package cmd

import (
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



func TestUpCommand_NoEditHosts(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantEditHosts bool
	}{
		{
			name:          "デフォルト（hosts編集する）",
			args:          []string{"-f", "testdata/test-services.yaml"},
			wantEditHosts: true,
		},
		{
			name:          "--no-edit-hostsでスキップ",
			args:          []string{"-f", "testdata/test-services.yaml", "--no-edit-hosts"},
			wantEditHosts: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// upOptsをリセット
			upOpts = &upOptions{
				noEditHosts: false, // デフォルト値
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
			cmd.Flags().BoolVar(&upOpts.noEditHosts, "no-edit-hosts", false, "skip updating /etc/hosts")
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err != nil {
				t.Errorf("Execute() error = %v", err)
			}

			// 論理反転の確認
			actualEditHosts := !upOpts.noEditHosts
			if actualEditHosts != tt.wantEditHosts {
				t.Errorf("editHosts = %v, want %v", actualEditHosts, tt.wantEditHosts)
			}
		})
	}
}

func TestMain(m *testing.M) {
	// テスト実行
	code := m.Run()
	os.Exit(code)
}
