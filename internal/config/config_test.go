package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultListenerPort(t *testing.T) {
	// listener_portを指定しない設定ファイル
	content := `
services:
  - host: test.localhost
    namespace: test
    service: test-svc
    port: 8080
    type: http
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	expectedPort := 80
	if cfg.ListenerPort != expectedPort {
		t.Errorf("expected default listener_port %d, got %d", expectedPort, cfg.ListenerPort)
	}
}

func TestLoad_ExplicitListenerPort(t *testing.T) {
	// listener_portを明示的に指定
	content := `
listener_port: 8080
services:
  - host: test.localhost
    namespace: test
    service: test-svc
    port: 8080
    type: http
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	expectedPort := 8080
	if cfg.ListenerPort != expectedPort {
		t.Errorf("expected listener_port %d, got %d", expectedPort, cfg.ListenerPort)
	}
}
