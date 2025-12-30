package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommand(t *testing.T) {
	t.Run("Execute returns no error", func(t *testing.T) {
		err := Execute()
		if err != nil {
			t.Errorf("Execute() returned error: %v", err)
		}
	})
}

func TestRootCommand_GlobalFlags(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantLogLevel string
	}{
		{
			name:         "デフォルトログレベル",
			args:         []string{},
			wantLogLevel: "info",
		},
		{
			name:         "グローバルフラグでdebug指定",
			args:         []string{"--log-level", "debug"},
			wantLogLevel: "debug",
		},
		{
			name:         "グローバルフラグでwarn指定",
			args:         []string{"--log-level", "warn"},
			wantLogLevel: "warn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalLogLevel = "info"

			cmd := &cobra.Command{Use: "test"}
			cmd.PersistentFlags().StringVar(&globalLogLevel, "log-level", "info", "log level")
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err != nil {
				t.Errorf("Execute() error = %v", err)
			}

			if globalLogLevel != tt.wantLogLevel {
				t.Errorf("globalLogLevel = %v, want %v", globalLogLevel, tt.wantLogLevel)
			}
		})
	}
}
