package run

import (
	"context"
	"testing"

	"github.com/usadamasa/kubectl-localmesh/internal/config"
	"github.com/usadamasa/kubectl-localmesh/internal/log"
)

func TestRunVisitor_Creation(t *testing.T) {
	// RunVisitorの生成テスト
	ctx := context.Background()
	cfg := &config.Config{}

	visitor := NewRunVisitor(ctx, cfg, log.New("info"), false)

	if visitor == nil {
		t.Fatal("expected visitor to be created")
	}

	if len(visitor.GetServiceConfigs()) != 0 {
		t.Errorf("expected 0 configs, got %d", len(visitor.GetServiceConfigs()))
	}

	if len(visitor.GetServiceSummaries()) != 0 {
		t.Errorf("expected 0 summaries, got %d", len(visitor.GetServiceSummaries()))
	}
}
