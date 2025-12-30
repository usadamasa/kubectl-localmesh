package cmd

import (
	"testing"
)

func TestRootCommand(t *testing.T) {
	t.Run("Execute returns no error", func(t *testing.T) {
		err := Execute()
		if err != nil {
			t.Errorf("Execute() returned error: %v", err)
		}
	})
}
