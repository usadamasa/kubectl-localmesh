package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDumpEnvoyConfigCommand_FlagParsing(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantConfig  string
		wantMock    string
		wantErr     bool
		errContains string
	}{
		{
			name:       "flag形式で設定ファイル指定",
			args:       []string{"-f", "testdata/test-services.yaml"},
			wantConfig: "testdata/test-services.yaml",
			wantMock:   "",
			wantErr:    false,
		},
		{
			name:       "long flag形式で設定ファイル指定",
			args:       []string{"--config", "testdata/test-services.yaml"},
			wantConfig: "testdata/test-services.yaml",
			wantMock:   "",
			wantErr:    false,
		},
		{
			name:       "位置引数で設定ファイル指定",
			args:       []string{"testdata/test-services.yaml"},
			wantConfig: "testdata/test-services.yaml",
			wantMock:   "",
			wantErr:    false,
		},
		{
			name:       "mock-config指定",
			args:       []string{"-f", "testdata/test-services.yaml", "--mock-config", "testdata/mocks.yaml"},
			wantConfig: "testdata/test-services.yaml",
			wantMock:   "testdata/mocks.yaml",
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
			dumpEnvoyConfigOpts = &dumpEnvoyConfigOptions{}

			cmd := &cobra.Command{
				Use: "test",
				RunE: func(cmd *cobra.Command, args []string) error {
					if dumpEnvoyConfigOpts.configFile == "" && len(args) > 0 {
						dumpEnvoyConfigOpts.configFile = args[0]
					}
					if dumpEnvoyConfigOpts.configFile == "" {
						return fmt.Errorf("config file required")
					}
					return nil
				},
			}

			cmd.Flags().StringVarP(&dumpEnvoyConfigOpts.configFile, "config", "f", "", "config yaml path")
			cmd.Flags().StringVar(&dumpEnvoyConfigOpts.mockConfig, "mock-config", "", "mock config path")
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

			if !tt.wantErr {
				if dumpEnvoyConfigOpts.configFile != tt.wantConfig {
					t.Errorf("configFile = %v, want %v", dumpEnvoyConfigOpts.configFile, tt.wantConfig)
				}
				if dumpEnvoyConfigOpts.mockConfig != tt.wantMock {
					t.Errorf("mockConfig = %v, want %v", dumpEnvoyConfigOpts.mockConfig, tt.wantMock)
				}
			}
		})
	}
}
