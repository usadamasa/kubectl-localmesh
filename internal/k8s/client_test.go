package k8s

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewClient_ValidKubeconfig(t *testing.T) {
	// 一時ディレクトリ作成（テスト終了時に自動削除）
	tmpDir := t.TempDir()

	// 最小限の有効なkubeconfigファイル作成
	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
    insecure-skip-tls-verify: true
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`

	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
	err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0600)
	if err != nil {
		t.Fatal(err)
	}

	// 環境変数を設定（テスト終了時に自動復元）
	t.Setenv("KUBECONFIG", kubeconfigPath)

	// テスト実行
	clientset, restConfig, err := NewClient()

	// アサーション
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if clientset == nil {
		t.Fatal("expected clientset to be non-nil")
	}
	if restConfig == nil {
		t.Fatal("expected restConfig to be non-nil")
	}

	// restConfigの基本的な検証
	if restConfig.Host != "https://127.0.0.1:6443" {
		t.Errorf("expected host 'https://127.0.0.1:6443', got %q", restConfig.Host)
	}
}

func TestNewClient_InvalidKubeconfig(t *testing.T) {
	tmpDir := t.TempDir()

	// 無効なYAMLファイル
	invalidKubeconfigPath := filepath.Join(tmpDir, "invalid-kubeconfig")
	err := os.WriteFile(invalidKubeconfigPath, []byte("invalid: yaml: content:"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("KUBECONFIG", invalidKubeconfigPath)

	_, _, err = NewClient()
	if err == nil {
		t.Fatal("expected error for invalid kubeconfig, got nil")
	}
}

func TestNewClient_NoKubeconfig(t *testing.T) {
	tmpDir := t.TempDir()

	// 存在しないパスを設定
	nonExistentPath := filepath.Join(tmpDir, "nonexistent-kubeconfig")
	t.Setenv("KUBECONFIG", nonExistentPath)
	t.Setenv("HOME", tmpDir) // ~/.kube/configも存在しないようにする

	_, _, err := NewClient()
	if err == nil {
		t.Fatal("expected error for non-existent kubeconfig, got nil")
	}
}
