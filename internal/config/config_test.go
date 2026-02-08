package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/usadamasa/kubectl-localmesh/internal/port"
)

func TestLoad_DefaultListenerPort(t *testing.T) {
	// listener_portを指定しない設定ファイル
	content := `
services:
  - kind: kubernetes
    host: test.localhost
    namespace: test
    service: test-svc
    port: 8080
    protocol: http
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

	expectedPort := port.ListenerPort(80)
	if cfg.ListenerPort != expectedPort {
		t.Errorf("expected default listener_port %d, got %d", expectedPort, cfg.ListenerPort)
	}
}

func TestLoad_ExplicitListenerPort(t *testing.T) {
	// listener_portを明示的に指定
	content := `
listener_port: 8080
services:
  - kind: kubernetes
    host: test.localhost
    namespace: test
    service: test-svc
    port: 8080
    protocol: http
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

	expectedPort := port.ListenerPort(8080)
	if cfg.ListenerPort != expectedPort {
		t.Errorf("expected listener_port %d, got %d", expectedPort, cfg.ListenerPort)
	}
}

func TestLoad_SSHBastionWithTCPService(t *testing.T) {
	// SSH Bastion経由のTCP接続設定
	content := `
listener_port: 80
ssh_bastions:
  primary:
    instance: bastion-1
    zone: asia-northeast1-a
    project: test-project
services:
  - kind: tcp
    host: db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
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

	// SSH Bastionの確認
	if len(cfg.SSHBastions) != 1 {
		t.Fatalf("expected 1 ssh_bastion, got %d", len(cfg.SSHBastions))
	}
	bastion, ok := cfg.SSHBastions["primary"]
	if !ok {
		t.Fatal("expected bastion 'primary' not found")
	}
	if bastion.Instance != "bastion-1" {
		t.Errorf("expected instance 'bastion-1', got '%s'", bastion.Instance)
	}
	if bastion.Zone != "asia-northeast1-a" {
		t.Errorf("expected zone 'asia-northeast1-a', got '%s'", bastion.Zone)
	}
	if bastion.Project != "test-project" {
		t.Errorf("expected project 'test-project', got '%s'", bastion.Project)
	}

	// Serviceの確認
	if len(cfg.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(cfg.Services))
	}
	svcDef := cfg.Services[0]
	tcpSvc, ok := svcDef.AsTCP()
	if !ok {
		t.Fatal("expected TCP service, got different type")
	}
	if tcpSvc.Host != "db.localhost" {
		t.Errorf("expected host 'db.localhost', got '%s'", tcpSvc.Host)
	}
	if tcpSvc.SSHBastion != "primary" {
		t.Errorf("expected ssh_bastion 'primary', got '%s'", tcpSvc.SSHBastion)
	}
	if tcpSvc.TargetHost != "10.0.0.1" {
		t.Errorf("expected target_host '10.0.0.1', got '%s'", tcpSvc.TargetHost)
	}
	if tcpSvc.TargetPort != 5432 {
		t.Errorf("expected target_port 5432, got %d", tcpSvc.TargetPort)
	}
}

func TestLoad_SSHBastionReferenceNotFound(t *testing.T) {
	// 存在しないbastion名を参照
	content := `
listener_port: 80
ssh_bastions:
  primary:
    instance: bastion-1
    zone: asia-northeast1-a
    project: test-project
services:
  - kind: tcp
    host: db.localhost
    ssh_bastion: nonexistent
    target_host: 10.0.0.1
    target_port: 5432
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for nonexistent bastion reference, got nil")
	}
	if !containsString(err.Error(), "ssh_bastion 'nonexistent' not found") {
		t.Errorf("expected error containing 'ssh_bastion 'nonexistent' not found', got '%s'", err.Error())
	}
}

func TestLoad_TCPServiceWithoutTargetHost(t *testing.T) {
	// kind=tcpだがtarget_hostが未指定
	content := `
listener_port: 80
ssh_bastions:
  primary:
    instance: bastion-1
    zone: asia-northeast1-a
    project: test-project
services:
  - kind: tcp
    host: db.localhost
    ssh_bastion: primary
    target_port: 5432
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing target_host, got nil")
	}
	if !containsString(err.Error(), "target_host is required") {
		t.Errorf("expected error containing 'target_host is required', got '%s'", err.Error())
	}
}

func TestLoad_TCPServiceWithoutTargetPort(t *testing.T) {
	// kind=tcpだがtarget_portが未指定
	content := `
listener_port: 80
ssh_bastions:
  primary:
    instance: bastion-1
    zone: asia-northeast1-a
    project: test-project
services:
  - kind: tcp
    host: db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing target_port, got nil")
	}
	if !containsString(err.Error(), "target_port is required") {
		t.Errorf("expected error containing 'target_port is required', got '%s'", err.Error())
	}
}

func TestLoad_K8sServiceWithoutNamespace(t *testing.T) {
	// kind=kubernetesだがnamespaceが未指定
	content := `
listener_port: 80
services:
  - kind: kubernetes
    host: api.localhost
    service: api-svc
    protocol: http
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing namespace, got nil")
	}
	if !containsString(err.Error(), "namespace is required") {
		t.Errorf("expected error containing 'namespace is required', got '%s'", err.Error())
	}
}

func TestLoad_MixedK8sAndTCPServices(t *testing.T) {
	// Kubernetes ServiceとTCP Serviceの混在
	content := `
listener_port: 80
ssh_bastions:
  primary:
    instance: bastion-1
    zone: asia-northeast1-a
    project: test-project
services:
  - kind: kubernetes
    host: api.localhost
    namespace: default
    service: api-svc
    protocol: grpc
  - kind: tcp
    host: db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
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

	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cfg.Services))
	}

	// Kubernetes Service
	k8sSvcDef := cfg.Services[0]
	k8sSvc, ok := k8sSvcDef.AsKubernetes()
	if !ok {
		t.Fatal("expected first service to be Kubernetes service")
	}
	if k8sSvc.Protocol != "grpc" {
		t.Errorf("expected protocol 'grpc', got '%s'", k8sSvc.Protocol)
	}
	if k8sSvc.Namespace != "default" {
		t.Errorf("expected namespace 'default', got '%s'", k8sSvc.Namespace)
	}
	if k8sSvc.Service != "api-svc" {
		t.Errorf("expected service 'api-svc', got '%s'", k8sSvc.Service)
	}

	// TCP Service
	tcpSvcDef := cfg.Services[1]
	tcpSvc, ok := tcpSvcDef.AsTCP()
	if !ok {
		t.Fatal("expected second service to be TCP service")
	}
	if tcpSvc.SSHBastion != "primary" {
		t.Errorf("expected ssh_bastion 'primary', got '%s'", tcpSvc.SSHBastion)
	}
	if tcpSvc.TargetHost != "10.0.0.1" {
		t.Errorf("expected target_host '10.0.0.1', got '%s'", tcpSvc.TargetHost)
	}
}

// ========== 新形式（kind: kubernetes / kind: tcp）のテスト ==========

func TestLoad_MissingKindField(t *testing.T) {
	// kindフィールドが存在しない場合
	content := `
listener_port: 80
services:
  - host: test.localhost
    namespace: test
    service: test-svc
    protocol: http
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing kind field, got nil")
	}
	if !containsString(err.Error(), "kind") {
		t.Errorf("expected error to contain 'kind', got '%s'", err.Error())
	}
}

func TestLoad_InvalidKindValue(t *testing.T) {
	// 不正なkind値
	content := `
listener_port: 80
services:
  - kind: invalid_kind
    host: test.localhost
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for invalid kind value, got nil")
	}
	if !containsString(err.Error(), "unknown service kind") && !containsString(err.Error(), "invalid_kind") {
		t.Errorf("expected error to contain 'unknown service kind' or 'invalid_kind', got '%s'", err.Error())
	}
}

func TestLoad_NewFormat_KubernetesServiceValid(t *testing.T) {
	// 新形式のKubernetesサービス（正常系）
	content := `
listener_port: 80
services:
  - kind: kubernetes
    host: test.localhost
    namespace: test
    service: test-svc
    protocol: http
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

	if len(cfg.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(cfg.Services))
	}

	svcDef := cfg.Services[0]
	k8sSvc, ok := svcDef.AsKubernetes()
	if !ok {
		t.Fatal("expected KubernetesService, got different type")
	}

	if k8sSvc.Host != "test.localhost" {
		t.Errorf("expected host 'test.localhost', got '%s'", k8sSvc.Host)
	}
	if k8sSvc.Namespace != "test" {
		t.Errorf("expected namespace 'test', got '%s'", k8sSvc.Namespace)
	}
	if k8sSvc.Service != "test-svc" {
		t.Errorf("expected service 'test-svc', got '%s'", k8sSvc.Service)
	}
	if k8sSvc.Protocol != "http" {
		t.Errorf("expected protocol 'http', got '%s'", k8sSvc.Protocol)
	}
}

func TestLoad_NewFormat_KubernetesServiceMissingNamespace(t *testing.T) {
	// 新形式のKubernetesサービス（namespace不足）
	content := `
listener_port: 80
services:
  - kind: kubernetes
    host: test.localhost
    service: test-svc
    protocol: http
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing namespace, got nil")
	}
	if !containsString(err.Error(), "namespace") && !containsString(err.Error(), "required") {
		t.Errorf("expected error to contain 'namespace' and 'required', got '%s'", err.Error())
	}
}

func TestLoad_NewFormat_KubernetesServiceInvalidProtocol(t *testing.T) {
	// 新形式のKubernetesサービス（不正なprotocol）
	content := `
listener_port: 80
services:
  - kind: kubernetes
    host: test.localhost
    namespace: test
    service: test-svc
    protocol: invalid
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for invalid protocol, got nil")
	}
	if !containsString(err.Error(), "protocol") {
		t.Errorf("expected error to contain 'protocol', got '%s'", err.Error())
	}
}

func TestLoad_NewFormat_TCPServiceValid(t *testing.T) {
	// 新形式のTCPサービス（正常系）
	content := `
listener_port: 80
ssh_bastions:
  primary:
    instance: bastion-1
    zone: asia-northeast1-a
    project: test-project
services:
  - kind: tcp
    host: db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
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

	if len(cfg.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(cfg.Services))
	}

	svcDef := cfg.Services[0]
	tcpSvc, ok := svcDef.AsTCP()
	if !ok {
		t.Fatal("expected TCPService, got different type")
	}

	if tcpSvc.Host != "db.localhost" {
		t.Errorf("expected host 'db.localhost', got '%s'", tcpSvc.Host)
	}
	if tcpSvc.SSHBastion != "primary" {
		t.Errorf("expected ssh_bastion 'primary', got '%s'", tcpSvc.SSHBastion)
	}
	if tcpSvc.TargetHost != "10.0.0.1" {
		t.Errorf("expected target_host '10.0.0.1', got '%s'", tcpSvc.TargetHost)
	}
	if tcpSvc.TargetPort != 5432 {
		t.Errorf("expected target_port 5432, got %d", tcpSvc.TargetPort)
	}
}

func TestLoad_NewFormat_TCPServiceMissingTargetHost(t *testing.T) {
	// 新形式のTCPサービス（target_host不足）
	content := `
listener_port: 80
ssh_bastions:
  primary:
    instance: bastion-1
    zone: asia-northeast1-a
services:
  - kind: tcp
    host: db.localhost
    ssh_bastion: primary
    target_port: 5432
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing target_host, got nil")
	}
	if !containsString(err.Error(), "target_host") && !containsString(err.Error(), "required") {
		t.Errorf("expected error to contain 'target_host' and 'required', got '%s'", err.Error())
	}
}

func TestLoad_NewFormat_MixedServicesValid(t *testing.T) {
	// 新形式の混在設定（正常系）
	content := `
listener_port: 80
ssh_bastions:
  primary:
    instance: bastion-1
    zone: asia-northeast1-a
    project: test-project
services:
  - kind: kubernetes
    host: api.localhost
    namespace: default
    service: api-svc
    protocol: grpc
  - kind: tcp
    host: db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
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

	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cfg.Services))
	}

	// Kubernetes Service
	k8sSvcDef := cfg.Services[0]
	k8sSvc, ok := k8sSvcDef.AsKubernetes()
	if !ok {
		t.Fatal("expected first service to be KubernetesService")
	}
	if k8sSvc.Protocol != "grpc" {
		t.Errorf("expected protocol 'grpc', got '%s'", k8sSvc.Protocol)
	}
	if k8sSvc.Namespace != "default" {
		t.Errorf("expected namespace 'default', got '%s'", k8sSvc.Namespace)
	}

	// TCP Service
	tcpSvcDef := cfg.Services[1]
	tcpSvc, ok := tcpSvcDef.AsTCP()
	if !ok {
		t.Fatal("expected second service to be TCPService")
	}
	if tcpSvc.SSHBastion != "primary" {
		t.Errorf("expected ssh_bastion 'primary', got '%s'", tcpSvc.SSHBastion)
	}
	if tcpSvc.TargetHost != "10.0.0.1" {
		t.Errorf("expected target_host '10.0.0.1', got '%s'", tcpSvc.TargetHost)
	}
}

func TestServiceDefinition_AsKubernetes(t *testing.T) {
	// KubernetesServiceの型アサーションテスト
	content := `
listener_port: 80
services:
  - kind: kubernetes
    host: test.localhost
    namespace: test
    service: test-svc
    protocol: http
  - kind: tcp
    host: db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// SSH bastionsを追加
	contentWithBastion := `
listener_port: 80
ssh_bastions:
  primary:
    instance: bastion-1
    zone: asia-northeast1-a
    project: test-project
services:
  - kind: kubernetes
    host: test.localhost
    namespace: test
    service: test-svc
    protocol: http
  - kind: tcp
    host: db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
`
	if err := os.WriteFile(configPath, []byte(contentWithBastion), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// KubernetesServiceはAsKubernetes()がtrueを返す
	k8sSvcDef := cfg.Services[0]
	if _, ok := k8sSvcDef.AsKubernetes(); !ok {
		t.Error("expected AsKubernetes() to return true for KubernetesService")
	}
	if _, ok := k8sSvcDef.AsTCP(); ok {
		t.Error("expected AsTCP() to return false for KubernetesService")
	}

	// TCPServiceはAsTCP()がtrueを返す
	tcpSvcDef := cfg.Services[1]
	if _, ok := tcpSvcDef.AsTCP(); !ok {
		t.Error("expected AsTCP() to return true for TCPService")
	}
	if _, ok := tcpSvcDef.AsKubernetes(); ok {
		t.Error("expected AsKubernetes() to return false for TCPService")
	}
}

// ヘルパー関数
func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// カバレッジ向上のための追加テスト

func TestServiceInterface_GetHostAndKind(t *testing.T) {
	// GetHost()とGetKind()メソッドのテスト
	content := `
listener_port: 80
ssh_bastions:
  primary:
    instance: bastion-1
    zone: asia-northeast1-a
    project: test-project
services:
  - kind: kubernetes
    host: k8s.localhost
    namespace: test
    service: test-svc
    protocol: http
  - kind: tcp
    host: db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
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

	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cfg.Services))
	}

	// Kubernetes Serviceのテスト
	k8sSvc := cfg.Services[0].Get()
	if k8sSvc.GetHost() != "k8s.localhost" {
		t.Errorf("expected host 'k8s.localhost', got '%s'", k8sSvc.GetHost())
	}
	if k8sSvc.GetKind() != "kubernetes" {
		t.Errorf("expected kind 'kubernetes', got '%s'", k8sSvc.GetKind())
	}

	// TCP Serviceのテスト
	tcpSvc := cfg.Services[1].Get()
	if tcpSvc.GetHost() != "db.localhost" {
		t.Errorf("expected host 'db.localhost', got '%s'", tcpSvc.GetHost())
	}
	if tcpSvc.GetKind() != "tcp" {
		t.Errorf("expected kind 'tcp', got '%s'", tcpSvc.GetKind())
	}
}

func TestServiceDefinition_MarshalYAML(t *testing.T) {
	// MarshalYAMLのテスト（KubernetesService）
	k8sSvc := &KubernetesService{
		Host:      "test.localhost",
		Namespace: "test",
		Service:   "test-svc",
		Protocol:  "http",
	}
	svcDef := ServiceDefinition{service: k8sSvc}

	data, err := svcDef.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML failed for KubernetesService: %v", err)
	}
	if data == nil {
		t.Error("expected non-nil marshaled data")
	}

	// MarshalYAMLのテスト（TCPService）
	tcpSvc := &TCPService{
		Host:       "db.localhost",
		SSHBastion: "primary",
		TargetHost: "10.0.0.1",
		TargetPort: 5432,
	}
	tcpSvcDef := ServiceDefinition{service: tcpSvc}

	tcpData, err := tcpSvcDef.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML failed for TCPService: %v", err)
	}
	if tcpData == nil {
		t.Error("expected non-nil marshaled data for TCP service")
	}
}

func TestLoadMockConfig_Valid(t *testing.T) {
	// LoadMockConfigのテスト
	content := `
mocks:
  - namespace: test
    service: test-svc
    port_name: http
    resolved_port: 8080
  - namespace: another
    service: another-svc
    port_name: grpc
    resolved_port: 50051
`
	tmpDir := t.TempDir()
	mockPath := filepath.Join(tmpDir, "mocks.yaml")
	if err := os.WriteFile(mockPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	mockCfg, err := LoadMockConfig(mockPath)
	if err != nil {
		t.Fatalf("LoadMockConfig failed: %v", err)
	}

	if mockCfg == nil {
		t.Fatal("expected non-nil MockConfig")
	}

	if len(mockCfg.Mocks) != 2 {
		t.Fatalf("expected 2 mocks, got %d", len(mockCfg.Mocks))
	}

	// 最初のモック
	if mockCfg.Mocks[0].Namespace != "test" {
		t.Errorf("expected namespace 'test', got '%s'", mockCfg.Mocks[0].Namespace)
	}
	if mockCfg.Mocks[0].Service != "test-svc" {
		t.Errorf("expected service 'test-svc', got '%s'", mockCfg.Mocks[0].Service)
	}
	if mockCfg.Mocks[0].ResolvedPort != 8080 {
		t.Errorf("expected resolved_port 8080, got %d", mockCfg.Mocks[0].ResolvedPort)
	}
}

func TestLoadMockConfig_EmptyPath(t *testing.T) {
	// 空パスの場合はnilを返す
	mockCfg, err := LoadMockConfig("")
	if err != nil {
		t.Fatalf("expected no error for empty path, got %v", err)
	}
	if mockCfg != nil {
		t.Error("expected nil MockConfig for empty path")
	}
}

func TestLoadMockConfig_InvalidFile(t *testing.T) {
	// 存在しないファイル
	_, err := LoadMockConfig("/nonexistent/path/to/mocks.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestLoadMockConfig_InvalidYAML(t *testing.T) {
	// 不正なYAMLファイル
	content := `
mocks:
  - namespace: test
    invalid yaml here
`
	tmpDir := t.TempDir()
	mockPath := filepath.Join(tmpDir, "mocks.yaml")
	if err := os.WriteFile(mockPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadMockConfig(mockPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestServiceDefinition_MarshalYAML_UnknownType(t *testing.T) {
	// 未知の型でMarshalYAMLを呼ぶ（nilサービス）
	svcDef := ServiceDefinition{service: nil}
	_, err := svcDef.MarshalYAML()
	if err == nil {
		t.Fatal("expected error for nil service, got nil")
	}
}

func TestKubernetesService_ValidateEdgeCases(t *testing.T) {
	// KubernetesServiceの全バリデーションパスをカバー
	cfg := &Config{}

	tests := []struct {
		name    string
		svc     *KubernetesService
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing host",
			svc:     &KubernetesService{Namespace: "test", Service: "svc", Protocol: "http"},
			wantErr: true,
			errMsg:  "host is required",
		},
		{
			name:    "missing service",
			svc:     &KubernetesService{Host: "test.localhost", Namespace: "test", Protocol: "http"},
			wantErr: true,
			errMsg:  "service is required",
		},
		{
			name:    "invalid protocol",
			svc:     &KubernetesService{Host: "test.localhost", Namespace: "test", Service: "svc", Protocol: "invalid"},
			wantErr: true,
			errMsg:  "protocol must be 'http', 'http2', or 'grpc'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.svc.Validate(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && !containsString(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
			}
		})
	}
}

func TestTCPService_ValidateEdgeCases(t *testing.T) {
	// TCPServiceの全バリデーションパスをカバー
	cfg := &Config{
		SSHBastions: map[string]*SSHBastion{
			"primary": {Instance: "test", Zone: "zone", Project: "proj"},
		},
	}

	tests := []struct {
		name    string
		svc     *TCPService
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing ssh_bastion",
			svc:     &TCPService{Host: "db.localhost", TargetHost: "10.0.0.1", TargetPort: 5432},
			wantErr: true,
			errMsg:  "ssh_bastion is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.svc.Validate(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && !containsString(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
			}
		})
	}
}

func TestTCPService_Validate_LocalhostWarning(t *testing.T) {
	// macOSでは.localhostサブドメインが/etc/hostsを無視して127.0.0.1に解決されるため警告を出す
	cfg := &Config{
		SSHBastions: map[string]*SSHBastion{
			"primary": {Instance: "test", Zone: "zone", Project: "proj"},
		},
	}

	tests := []struct {
		name           string
		host           string
		expectsWarning bool
	}{
		{"db.localhost triggers warning", "db.localhost", true},
		{"db.sub.localhost triggers warning", "db.sub.localhost", true},
		{"db.localdomain no warning", "db.localdomain", false},
		{"db.local no warning", "db.local", false},
		{"localhost itself no warning", "localhost", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			origWriter := port.WarnWriter()
			port.SetWarnWriter(&buf)
			defer port.SetWarnWriter(origWriter)

			svc := &TCPService{
				Host:       tt.host,
				SSHBastion: "primary",
				TargetHost: "10.0.0.1",
				TargetPort: 5432,
			}
			_ = svc.Validate(cfg)

			hasWarning := buf.Len() > 0
			if hasWarning != tt.expectsWarning {
				t.Errorf("host=%s: hasWarning=%v, expectsWarning=%v, output=%q",
					tt.host, hasWarning, tt.expectsWarning, buf.String())
			}
			if tt.expectsWarning && hasWarning {
				if !containsString(buf.String(), ".localhost") {
					t.Errorf("warning should mention .localhost, got: %s", buf.String())
				}
				if !containsString(buf.String(), "macOS") {
					t.Errorf("warning should mention macOS, got: %s", buf.String())
				}
			}
		})
	}
}

func TestLoad_InvalidFile(t *testing.T) {
	// 存在しないファイル
	_, err := Load("/nonexistent/path/to/config.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestLoad_NoServices(t *testing.T) {
	// servicesが空の場合
	content := `
listener_port: 80
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for no services, got nil")
	}
	if !containsString(err.Error(), "no services configured") {
		t.Errorf("expected error containing 'no services configured', got '%s'", err.Error())
	}
}

// ========== Cluster関連テスト ==========

func TestLoad_GlobalCluster(t *testing.T) {
	content := `
cluster: gke_myproject_asia-northeast1_staging
services:
  - kind: kubernetes
    host: test.localhost
    namespace: test
    service: test-svc
    port: 8080
    protocol: http
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

	if cfg.Cluster != "gke_myproject_asia-northeast1_staging" {
		t.Errorf("expected cluster 'gke_myproject_asia-northeast1_staging', got '%s'", cfg.Cluster)
	}
}

func TestLoad_PerServiceCluster(t *testing.T) {
	content := `
cluster: gke_myproject_asia-northeast1_staging
services:
  - kind: kubernetes
    host: api.localhost
    namespace: default
    service: api-svc
    protocol: http
  - kind: kubernetes
    host: admin.localhost
    namespace: admin
    service: admin-web
    protocol: http
    cluster: gke_myproject_asia-northeast1_prod
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

	// 1番目のサービスはcluster未指定
	svc1, ok := cfg.Services[0].AsKubernetes()
	if !ok {
		t.Fatal("expected KubernetesService for first service")
	}
	if svc1.Cluster != "" {
		t.Errorf("expected empty cluster for first service, got '%s'", svc1.Cluster)
	}

	// 2番目のサービスはcluster指定あり
	svc2, ok := cfg.Services[1].AsKubernetes()
	if !ok {
		t.Fatal("expected KubernetesService for second service")
	}
	if svc2.Cluster != "gke_myproject_asia-northeast1_prod" {
		t.Errorf("expected cluster 'gke_myproject_asia-northeast1_prod', got '%s'", svc2.Cluster)
	}
}

func TestLoad_ClusterTrimming(t *testing.T) {
	content := `
cluster: "  gke_myproject_asia-northeast1_staging  "
services:
  - kind: kubernetes
    host: test.localhost
    namespace: test
    service: test-svc
    protocol: http
    cluster: "  gke_myproject_asia-northeast1_prod  "
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

	if cfg.Cluster != "gke_myproject_asia-northeast1_staging" {
		t.Errorf("expected trimmed global cluster, got '%s'", cfg.Cluster)
	}

	svc, ok := cfg.Services[0].AsKubernetes()
	if !ok {
		t.Fatal("expected KubernetesService")
	}
	if svc.Cluster != "gke_myproject_asia-northeast1_prod" {
		t.Errorf("expected trimmed service cluster, got '%s'", svc.Cluster)
	}
}

func TestLoad_NoCluster(t *testing.T) {
	// cluster未指定の場合はデフォルト値（空文字列）
	content := `
services:
  - kind: kubernetes
    host: test.localhost
    namespace: test
    service: test-svc
    protocol: http
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

	if cfg.Cluster != "" {
		t.Errorf("expected empty cluster, got '%s'", cfg.Cluster)
	}
}

// ========== ListenerPort関連テスト ==========

func TestLoad_KubernetesService_WithListenerPort_GRPC(t *testing.T) {
	// gRPCサービスでlistener_portを指定（正常系）
	content := `
listener_port: 80
services:
  - kind: kubernetes
    host: grpc.localhost
    namespace: default
    service: grpc-svc
    port_name: grpc
    protocol: grpc
    listener_port: 50051
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

	if len(cfg.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(cfg.Services))
	}

	k8sSvc, ok := cfg.Services[0].AsKubernetes()
	if !ok {
		t.Fatal("expected KubernetesService")
	}
	if k8sSvc.ListenerPort != 50051 {
		t.Errorf("expected listener_port 50051, got %d", k8sSvc.ListenerPort)
	}
}

func TestLoad_KubernetesService_WithListenerPort_HTTP2(t *testing.T) {
	// http2サービスでlistener_portを指定（正常系）
	content := `
listener_port: 80
services:
  - kind: kubernetes
    host: http2.localhost
    namespace: default
    service: http2-svc
    protocol: http2
    listener_port: 8443
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

	k8sSvc, ok := cfg.Services[0].AsKubernetes()
	if !ok {
		t.Fatal("expected KubernetesService")
	}
	if k8sSvc.ListenerPort != 8443 {
		t.Errorf("expected listener_port 8443, got %d", k8sSvc.ListenerPort)
	}
}

func TestLoad_KubernetesService_WithListenerPort_HTTP(t *testing.T) {
	// httpサービスでlistener_portを指定（正常系）
	content := `
listener_port: 80
services:
  - kind: kubernetes
    host: http.localhost
    namespace: default
    service: http-svc
    protocol: http
    listener_port: 8080
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

	k8sSvc, ok := cfg.Services[0].AsKubernetes()
	if !ok {
		t.Fatal("expected KubernetesService")
	}
	if k8sSvc.ListenerPort != 8080 {
		t.Errorf("expected listener_port 8080, got %d", k8sSvc.ListenerPort)
	}
}

func TestLoad_KubernetesService_WithListenerPort_InvalidRange(t *testing.T) {
	// 不正なポート範囲（65536）
	content := `
listener_port: 80
services:
  - kind: kubernetes
    host: grpc.localhost
    namespace: default
    service: grpc-svc
    protocol: grpc
    listener_port: 65536
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for listener_port out of range, got nil")
	}
	if !containsString(err.Error(), "listener_port") {
		t.Errorf("expected error containing 'listener_port', got '%s'", err.Error())
	}
}

func TestLoad_KubernetesService_WithoutListenerPort(t *testing.T) {
	// listener_port省略（従来動作）
	content := `
listener_port: 80
services:
  - kind: kubernetes
    host: grpc.localhost
    namespace: default
    service: grpc-svc
    protocol: grpc
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

	k8sSvc, ok := cfg.Services[0].AsKubernetes()
	if !ok {
		t.Fatal("expected KubernetesService")
	}
	if k8sSvc.ListenerPort != 0 {
		t.Errorf("expected listener_port 0 (omitted), got %d", k8sSvc.ListenerPort)
	}
}

func TestLoad_KubernetesService_MultipleGRPCWithListenerPort(t *testing.T) {
	// 複数のgRPCサービスがそれぞれ異なるlistener_portを持つ
	content := `
listener_port: 80
services:
  - kind: kubernetes
    host: grpc1.localhost
    namespace: default
    service: grpc1-svc
    protocol: grpc
    listener_port: 50051
  - kind: kubernetes
    host: grpc2.localhost
    namespace: default
    service: grpc2-svc
    protocol: grpc
    listener_port: 50052
  - kind: kubernetes
    host: grpc3.localhost
    namespace: default
    service: grpc3-svc
    protocol: grpc
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

	if len(cfg.Services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(cfg.Services))
	}

	// 1番目: listener_port 50051
	grpc1, ok := cfg.Services[0].AsKubernetes()
	if !ok {
		t.Fatal("expected KubernetesService for first service")
	}
	if grpc1.ListenerPort != 50051 {
		t.Errorf("expected listener_port 50051, got %d", grpc1.ListenerPort)
	}

	// 2番目: listener_port 50052
	grpc2, ok := cfg.Services[1].AsKubernetes()
	if !ok {
		t.Fatal("expected KubernetesService for second service")
	}
	if grpc2.ListenerPort != 50052 {
		t.Errorf("expected listener_port 50052, got %d", grpc2.ListenerPort)
	}

	// 3番目: listener_port 省略（0）
	grpc3, ok := cfg.Services[2].AsKubernetes()
	if !ok {
		t.Fatal("expected KubernetesService for third service")
	}
	if grpc3.ListenerPort != 0 {
		t.Errorf("expected listener_port 0, got %d", grpc3.ListenerPort)
	}
}

// ========== SSH Bastion新フィールド関連テスト ==========

func TestLoad_SSHBastion_WithSSHKeyPath(t *testing.T) {
	// ssh_key_pathが正しく読み込まれること
	content := `
listener_port: 80
ssh_bastions:
  primary:
    instance: bastion-1
    zone: asia-northeast1-a
    project: test-project
    ssh_key_path: /home/user/.ssh/id_rsa
services:
  - kind: tcp
    host: db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
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

	bastion, ok := cfg.SSHBastions["primary"]
	if !ok {
		t.Fatal("expected bastion 'primary' not found")
	}
	if bastion.SSHKeyPath != "/home/user/.ssh/id_rsa" {
		t.Errorf("expected ssh_key_path '/home/user/.ssh/id_rsa', got '%s'", bastion.SSHKeyPath)
	}
}

func TestLoad_SSHBastion_WithSSHUser(t *testing.T) {
	// ssh_userが正しく読み込まれること
	content := `
listener_port: 80
ssh_bastions:
  primary:
    instance: bastion-1
    zone: asia-northeast1-a
    project: test-project
    ssh_user: gce-user
services:
  - kind: tcp
    host: db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
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

	bastion, ok := cfg.SSHBastions["primary"]
	if !ok {
		t.Fatal("expected bastion 'primary' not found")
	}
	if bastion.SSHUser != "gce-user" {
		t.Errorf("expected ssh_user 'gce-user', got '%s'", bastion.SSHUser)
	}
}

func TestLoad_SSHBastion_ProjectFallback_GOOGLE_CLOUD_PROJECT(t *testing.T) {
	// 環境変数フォールバック: GOOGLE_CLOUD_PROJECT
	t.Setenv("GOOGLE_CLOUD_PROJECT", "fallback-project-1")
	t.Setenv("GCLOUD_PROJECT", "fallback-project-2")
	t.Setenv("CLOUDSDK_CORE_PROJECT", "fallback-project-3")

	content := `
listener_port: 80
ssh_bastions:
  primary:
    instance: bastion-1
    zone: asia-northeast1-a
services:
  - kind: tcp
    host: db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
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

	bastion, ok := cfg.SSHBastions["primary"]
	if !ok {
		t.Fatal("expected bastion 'primary' not found")
	}
	if bastion.Project != "fallback-project-1" {
		t.Errorf("expected project 'fallback-project-1' (from GOOGLE_CLOUD_PROJECT), got '%s'", bastion.Project)
	}
}

func TestLoad_SSHBastion_ProjectFallback_CLOUDSDK_CORE_PROJECT(t *testing.T) {
	// 環境変数フォールバック: CLOUDSDK_CORE_PROJECT（GOOGLE_CLOUD_PROJECTが未設定）
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GCLOUD_PROJECT", "")
	t.Setenv("CLOUDSDK_CORE_PROJECT", "fallback-project-3")

	content := `
listener_port: 80
ssh_bastions:
  primary:
    instance: bastion-1
    zone: asia-northeast1-a
services:
  - kind: tcp
    host: db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
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

	bastion, ok := cfg.SSHBastions["primary"]
	if !ok {
		t.Fatal("expected bastion 'primary' not found")
	}
	if bastion.Project != "fallback-project-3" {
		t.Errorf("expected project 'fallback-project-3' (from CLOUDSDK_CORE_PROJECT), got '%s'", bastion.Project)
	}
}

func TestLoad_SSHBastion_ProjectRequired(t *testing.T) {
	// TCPサービスがある場合にprojectが必須
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GCLOUD_PROJECT", "")
	t.Setenv("CLOUDSDK_CORE_PROJECT", "")

	content := `
listener_port: 80
ssh_bastions:
  primary:
    instance: bastion-1
    zone: asia-northeast1-a
services:
  - kind: tcp
    host: db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing project when TCP service exists, got nil")
	}
	if !containsString(err.Error(), "project is required") {
		t.Errorf("expected error containing 'project is required', got '%s'", err.Error())
	}
}
