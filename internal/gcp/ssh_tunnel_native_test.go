package gcp

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/usadamasa/kubectl-localmesh/internal/config"
	"github.com/usadamasa/kubectl-localmesh/internal/log"
	"github.com/usadamasa/kubectl-localmesh/internal/port"
)

func TestStartGCPSSHTunnelNative_BasicFlow(t *testing.T) {
	// 基本的なSSH tunnel起動のテスト
	// ADCやIAP接続が利用できない環境では初回接続でエラーが返る
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	bastion := &config.SSHBastion{
		Instance: "test-instance",
		Zone:     "asia-northeast1-a",
		Project:  "test-project",
	}

	localPort := port.LocalPort(10000)
	targetHost := "10.0.0.1"
	targetPort := port.TCPPort(5432)

	// バリデーションを通過して内部実装に到達することを確認
	err := StartGCPSSHTunnelNative(ctx, bastion, localPort, targetHost, targetPort, log.New("info"))

	// ADC/IAP接続が利用できない環境ではエラーが返る（初回失敗で即終了）
	// contextタイムアウトの場合はnil
	if err == nil && ctx.Err() == nil {
		t.Error("expected error in non-ADC environment, got nil")
	}
}

func TestStartGCPSSHTunnelNative_InvalidBastion(t *testing.T) {
	// Bastionパラメータのバリデーション
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	bastion := &config.SSHBastion{
		Instance: "", // 空のinstance名
		Zone:     "asia-northeast1-a",
		Project:  "test-project",
	}

	err := StartGCPSSHTunnelNative(ctx, bastion, 10000, "10.0.0.1", 5432, log.New("info"))

	if err == nil {
		t.Error("expected error for empty instance name, got nil")
	}
}

func TestStartGCPSSHTunnelNative_NilBastion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := StartGCPSSHTunnelNative(ctx, nil, 10000, "10.0.0.1", 5432, log.New("info"))

	if err == nil {
		t.Error("expected error for nil bastion, got nil")
	}
}

func TestStartGCPSSHTunnelNative_EmptyZone(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	bastion := &config.SSHBastion{
		Instance: "test-instance",
		Zone:     "", // 空のzone
		Project:  "test-project",
	}

	err := StartGCPSSHTunnelNative(ctx, bastion, 10000, "10.0.0.1", 5432, log.New("info"))

	if err == nil {
		t.Error("expected error for empty zone, got nil")
	}
}

func TestStartGCPSSHTunnelNative_InvalidPorts(t *testing.T) {
	// ポート番号のバリデーション
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	bastion := &config.SSHBastion{
		Instance: "test-instance",
		Zone:     "asia-northeast1-a",
		Project:  "test-project",
	}

	// localPort が0
	err := StartGCPSSHTunnelNative(ctx, bastion, 0, "10.0.0.1", 5432, log.New("info"))
	if err == nil {
		t.Error("expected error for invalid local port, got nil")
	}

	// targetPort が0
	err = StartGCPSSHTunnelNative(ctx, bastion, 10000, "10.0.0.1", 0, log.New("info"))
	if err == nil {
		t.Error("expected error for invalid target port, got nil")
	}
}

func TestStartGCPSSHTunnelNative_InvalidTargetHost(t *testing.T) {
	// ターゲットホストのバリデーション
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	bastion := &config.SSHBastion{
		Instance: "test-instance",
		Zone:     "asia-northeast1-a",
		Project:  "test-project",
	}

	err := StartGCPSSHTunnelNative(ctx, bastion, 10000, "", 5432, log.New("info"))
	if err == nil {
		t.Error("expected error for empty target host, got nil")
	}
}

func TestForwardConnection(t *testing.T) {
	// forwardConnectionの基本動作テスト
	// net.Pipeで擬似的な接続ペアを作成
	local1, local2 := net.Pipe()
	remote1, remote2 := net.Pipe()

	go forwardConnection(local1, remote1)

	// local2 → local1 → remote1 → remote2 へのデータ転送を確認
	testData := []byte("hello, tunnel!")
	_, err := local2.Write(testData)
	if err != nil {
		t.Fatalf("failed to write test data: %v", err)
	}

	buf := make([]byte, len(testData))
	_, err = remote2.Read(buf)
	if err != nil {
		t.Fatalf("failed to read forwarded data: %v", err)
	}

	if string(buf) != string(testData) {
		t.Errorf("expected %q, got %q", string(testData), string(buf))
	}

	_ = local2.Close()
	_ = remote2.Close()
}
