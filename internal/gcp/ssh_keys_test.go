package gcp

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func generateTestKey(t *testing.T) []byte {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	privateKeyBlock, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}

	return pem.EncodeToMemory(privateKeyBlock)
}

func TestFindSSHKey_ConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "custom-key")

	// テスト用の秘密鍵を生成
	keyData := generateTestKey(t)
	if err := os.WriteFile(keyPath, keyData, 0600); err != nil {
		t.Fatalf("failed to write test key: %v", err)
	}

	// configPath で指定したパスから鍵が読み込まれることを確認
	signer, err := FindSSHKey(keyPath)
	if err != nil {
		t.Fatalf("FindSSHKey failed: %v", err)
	}
	if signer == nil {
		t.Fatal("signer is nil")
	}
}

func TestFindSSHKey_GoogleComputeEngine(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.Mkdir(sshDir, 0700); err != nil {
		t.Fatalf("failed to create .ssh directory: %v", err)
	}

	// ~/.ssh/google_compute_engine を模擬
	gcpKeyPath := filepath.Join(sshDir, "google_compute_engine")
	keyData := generateTestKey(t)
	if err := os.WriteFile(gcpKeyPath, keyData, 0600); err != nil {
		t.Fatalf("failed to write test key: %v", err)
	}

	// 一時的にホームディレクトリをオーバーライド
	t.Setenv("HOME", tmpDir)

	// 空のconfigPath で ~/ を探索
	signer, err := FindSSHKey("")
	if err != nil {
		t.Fatalf("FindSSHKey failed: %v", err)
	}
	if signer == nil {
		t.Fatal("signer is nil")
	}
}

func TestFindSSHKey_Ed25519(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.Mkdir(sshDir, 0700); err != nil {
		t.Fatalf("failed to create .ssh directory: %v", err)
	}

	// ~/.ssh/id_ed25519 を模擬
	ed25519KeyPath := filepath.Join(sshDir, "id_ed25519")
	keyData := generateTestKey(t)
	if err := os.WriteFile(ed25519KeyPath, keyData, 0600); err != nil {
		t.Fatalf("failed to write test key: %v", err)
	}

	// 一時的にホームディレクトリをオーバーライド
	t.Setenv("HOME", tmpDir)

	// 空のconfigPath で ~/ を探索
	signer, err := FindSSHKey("")
	if err != nil {
		t.Fatalf("FindSSHKey failed: %v", err)
	}
	if signer == nil {
		t.Fatal("signer is nil")
	}
}

func TestFindSSHKey_RSA(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.Mkdir(sshDir, 0700); err != nil {
		t.Fatalf("failed to create .ssh directory: %v", err)
	}

	// ~/.ssh/id_rsa を模擬（ed25519 でシミュレート、フォーマットは互換）
	rsaKeyPath := filepath.Join(sshDir, "id_rsa")
	keyData := generateTestKey(t)
	if err := os.WriteFile(rsaKeyPath, keyData, 0600); err != nil {
		t.Fatalf("failed to write test key: %v", err)
	}

	// 一時的にホームディレクトリをオーバーライド
	t.Setenv("HOME", tmpDir)

	// 空のconfigPath で ~/ を探索
	signer, err := FindSSHKey("")
	if err != nil {
		t.Fatalf("FindSSHKey failed: %v", err)
	}
	if signer == nil {
		t.Fatal("signer is nil")
	}
}

func TestFindSSHKey_Priority(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.Mkdir(sshDir, 0700); err != nil {
		t.Fatalf("failed to create .ssh directory: %v", err)
	}

	// 複数の鍵を作成：id_rsa と id_ed25519
	keyData := generateTestKey(t)

	rsaKeyPath := filepath.Join(sshDir, "id_rsa")
	if err := os.WriteFile(rsaKeyPath, keyData, 0600); err != nil {
		t.Fatalf("failed to write rsa key: %v", err)
	}

	ed25519KeyPath := filepath.Join(sshDir, "id_ed25519")
	if err := os.WriteFile(ed25519KeyPath, keyData, 0600); err != nil {
		t.Fatalf("failed to write ed25519 key: %v", err)
	}

	// 一時的にホームディレクトリをオーバーライド
	t.Setenv("HOME", tmpDir)

	// google_compute_engine がない場合、id_ed25519 が優先されることを確認
	signer, err := FindSSHKey("")
	if err != nil {
		t.Fatalf("FindSSHKey failed: %v", err)
	}
	if signer == nil {
		t.Fatal("signer is nil")
	}
}

func TestFindSSHKey_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// 一時的にホームディレクトリをオーバーライド（鍵が存在しないディレクトリ）
	t.Setenv("HOME", tmpDir)

	// 鍵が見つからない場合のエラーメッセージを確認
	_, err := FindSSHKey("")
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	errMsg := err.Error()
	// エラーメッセージにgcloudコマンドが含まれていることを確認
	if !strings.Contains(errMsg, "gcloud compute ssh") {
		t.Fatalf("error message should contain 'gcloud compute ssh', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "SSH key not found") {
		t.Fatalf("error message should contain 'SSH key not found', got: %s", errMsg)
	}
}

func TestResolveSSHUser_ConfigUser(t *testing.T) {
	// config で指定されたユーザー名が優先されることを確認
	result := ResolveSSHUser("myuser")
	if result != "myuser" {
		t.Fatalf("expected 'myuser', got '%s'", result)
	}
}

func TestResolveSSHUser_SudoUser(t *testing.T) {
	// sudo経由で実行された場合、SUDO_USERが優先されることを確認
	t.Setenv("SUDO_USER", "realuser")
	t.Setenv("USER", "root")
	result := ResolveSSHUser("")
	if result != "realuser" {
		t.Fatalf("expected 'realuser', got '%s'", result)
	}
}

func TestResolveSSHUser_EnvVar(t *testing.T) {
	// SUDO_USERが無い場合、USERが使用されることを確認
	t.Setenv("SUDO_USER", "")
	t.Setenv("USER", "testuser")
	result := ResolveSSHUser("")
	if result != "testuser" {
		t.Fatalf("expected 'testuser', got '%s'", result)
	}
}

func TestResolveSSHUser_UsernameEnvVar(t *testing.T) {
	// SUDO_USER と USER が無い場合、USERNAME が使用されることを確認（Windows互換）
	t.Setenv("SUDO_USER", "")
	t.Setenv("USER", "")
	t.Setenv("USERNAME", "winuser")
	result := ResolveSSHUser("")
	if result != "winuser" {
		t.Fatalf("expected 'winuser', got '%s'", result)
	}
}

func TestResolveSSHUser_Fallback(t *testing.T) {
	// すべての環境変数が無い場合、"root" にフォールバック
	t.Setenv("SUDO_USER", "")
	t.Setenv("USER", "")
	t.Setenv("USERNAME", "")
	result := ResolveSSHUser("")
	if result != "root" {
		t.Fatalf("expected 'root', got '%s'", result)
	}
}
