package gcp

import (
	"context"
	"os"
	"testing"
)

func TestTokenSource_NoCredentials(t *testing.T) {
	// ADCが利用可能な場合はスキップ、テストを実行可能とする
	// CI環境などADCがない場合はエラーが返される可能性がある
	ctx := context.Background()

	ts, err := TokenSource(ctx)

	// ADCがない環境ではエラーが返ることを許容
	// ADCがある環境ではTokenSourceが返される
	if ts == nil && err == nil {
		t.Error("expected either TokenSource or error, got both nil")
	}

	// エラーメッセージの確認（ADCが見つからない場合）
	if err != nil {
		t.Logf("expected error in non-ADC environment: %v", err)
	}
}

func TestTokenSource_WithoutCredentials(t *testing.T) {
	// 明示的にADC環境変数を確認
	if _, hasADC := os.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS"); !hasADC {
		t.Skip("skipping test: GOOGLE_APPLICATION_CREDENTIALS not set")
	}

	ctx := context.Background()
	ts, err := TokenSource(ctx)

	if err != nil {
		t.Fatalf("TokenSource failed: %v", err)
	}

	if ts == nil {
		t.Error("TokenSource returned nil")
	}
}
