package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/usadamasa/kubectl-localmesh/internal/port"
)

type Config struct {
	ListenerPort port.ListenerPort      `yaml:"listener_port"`
	Cluster      string                 `yaml:"cluster,omitempty"`
	SSHBastions  map[string]*SSHBastion `yaml:"ssh_bastions,omitempty"`
	Services     []ServiceDefinition    `yaml:"services"`
}

type SSHBastion struct {
	Instance   string `yaml:"instance"`               // GCP Compute Instance名
	Zone       string `yaml:"zone"`                   // GCPゾーン
	Project    string `yaml:"project"`                // GCPプロジェクトID（省略時は環境変数でフォールバック）
	SSHKeyPath string `yaml:"ssh_key_path,omitempty"` // SSH秘密鍵パス
	SSHUser    string `yaml:"ssh_user,omitempty"`     // SSHユーザー名
}

// Service はすべてのサービス種別が実装すべきインターフェース
type Service interface {
	GetHost() string
	GetKind() string
	Validate(*Config) error
	Accept(ServiceVisitor) error
}

// ServiceDefinition はタグ付きユニオン型のルート構造体
type ServiceDefinition struct {
	service Service
}

// KubernetesService はKubernetes Service（HTTP/gRPC）を表現
type KubernetesService struct {
	Host         string            `yaml:"host"`
	Namespace    string            `yaml:"namespace"`
	Service      string            `yaml:"service"`
	PortName     string            `yaml:"port_name,omitempty"`
	Port         port.ServicePort  `yaml:"port,omitempty"`
	Protocol     string            `yaml:"protocol"`                // http|http2|grpc
	ListenerPort port.ListenerPort `yaml:"listener_port,omitempty"` // 個別リスナーポート（指定時はHTTPリスナーを上書き）
	Cluster      string            `yaml:"cluster,omitempty"`       // kubeconfig cluster name（オーバーライド用）
}

// TCPService はGCP SSH Bastion経由のTCP接続を表現
type TCPService struct {
	Host       string       `yaml:"host"`
	SSHBastion string       `yaml:"ssh_bastion"`
	TargetHost string       `yaml:"target_host"`
	TargetPort port.TCPPort `yaml:"target_port"`
	ListenPort port.TCPPort `yaml:"listen_port,omitempty"` // 省略時はTargetPortと同じ
}

// インターフェース実装
func (k *KubernetesService) GetHost() string { return k.Host }
func (k *KubernetesService) GetKind() string { return "kubernetes" }
func (t *TCPService) GetHost() string        { return t.Host }
func (t *TCPService) GetKind() string        { return "tcp" }

// Accept implements Service interface for Visitor pattern
func (k *KubernetesService) Accept(visitor ServiceVisitor) error {
	return visitor.VisitKubernetes(k)
}

// Accept implements Service interface for Visitor pattern
func (t *TCPService) Accept(visitor ServiceVisitor) error {
	return visitor.VisitTCP(t)
}

// Get は内部のServiceインターフェースを取得
func (sd *ServiceDefinition) Get() Service {
	return sd.service
}

// AsKubernetes は型アサーション（type switchの代替）
func (sd *ServiceDefinition) AsKubernetes() (*KubernetesService, bool) {
	k8s, ok := sd.service.(*KubernetesService)
	return k8s, ok
}

// AsTCP は型アサーション（type switchの代替）
func (sd *ServiceDefinition) AsTCP() (*TCPService, bool) {
	tcp, ok := sd.service.(*TCPService)
	return tcp, ok
}

// UnmarshalYAML でタグ付きユニオン型を実現
func (sd *ServiceDefinition) UnmarshalYAML(node *yaml.Node) error {
	// 1. まず汎用マップとしてデコード
	var raw map[string]interface{}
	if err := node.Decode(&raw); err != nil {
		return err
	}

	// 2. kindフィールドで型を判別
	kindRaw, ok := raw["kind"]
	if !ok {
		return fmt.Errorf("service must have 'kind' field")
	}
	kind, ok := kindRaw.(string)
	if !ok {
		return fmt.Errorf("'kind' must be a string")
	}

	// 3. kindに応じた構造体にデコード
	switch kind {
	case "kubernetes":
		var k8sSvc KubernetesService
		if err := node.Decode(&k8sSvc); err != nil {
			return err
		}
		sd.service = &k8sSvc
	case "tcp":
		var tcpSvc TCPService
		if err := node.Decode(&tcpSvc); err != nil {
			return err
		}
		sd.service = &tcpSvc
	default:
		return fmt.Errorf("unknown service kind: %s (must be 'kubernetes' or 'tcp')", kind)
	}

	return nil
}

// MarshalYAML でシリアライズ時にkindを自動付与
func (sd *ServiceDefinition) MarshalYAML() (interface{}, error) {
	type Alias struct {
		Kind string `yaml:"kind"`
	}

	switch svc := sd.service.(type) {
	case *KubernetesService:
		return struct {
			Alias
			*KubernetesService `yaml:",inline"`
		}{
			Alias:             Alias{Kind: "kubernetes"},
			KubernetesService: svc,
		}, nil
	case *TCPService:
		return struct {
			Alias
			*TCPService `yaml:",inline"`
		}{
			Alias:      Alias{Kind: "tcp"},
			TCPService: svc,
		}, nil
	default:
		return nil, fmt.Errorf("unknown service type: %T", svc)
	}
}

// バリデーション実装
func (k *KubernetesService) Validate(cfg *Config) error {
	if k.Host == "" {
		return fmt.Errorf("host is required for kubernetes service")
	}
	if k.Namespace == "" {
		return fmt.Errorf("namespace is required for kubernetes service '%s'", k.Host)
	}
	if k.Service == "" {
		return fmt.Errorf("service is required for kubernetes service '%s'", k.Host)
	}
	if k.Protocol != "http" && k.Protocol != "http2" && k.Protocol != "grpc" {
		return fmt.Errorf("protocol must be 'http', 'http2', or 'grpc' for kubernetes service '%s', got '%s'", k.Host, k.Protocol)
	}

	// ListenerPortのバリデーション（共通関数使用）
	if k.ListenerPort != 0 {
		if err := port.ValidatePort(k.ListenerPort, "listener_port", k.Host); err != nil {
			return err
		}
		port.WarnPrivilegedPort(k.ListenerPort, "listener_port", k.Host)
	}

	return nil
}

func (t *TCPService) Validate(cfg *Config) error {
	if t.Host == "" {
		return fmt.Errorf("host is required for tcp service")
	}
	if t.SSHBastion == "" {
		return fmt.Errorf("ssh_bastion is required for tcp service '%s'", t.Host)
	}
	bastion, ok := cfg.SSHBastions[t.SSHBastion]
	if !ok {
		return fmt.Errorf("ssh_bastion '%s' not found for service '%s'", t.SSHBastion, t.Host)
	}
	if bastion.Project == "" {
		return fmt.Errorf("project is required for tcp service '%s' (ssh_bastion '%s'): set in config or via GOOGLE_CLOUD_PROJECT, GCLOUD_PROJECT, or CLOUDSDK_CORE_PROJECT", t.Host, t.SSHBastion)
	}
	if t.TargetHost == "" {
		return fmt.Errorf("target_host is required for tcp service '%s'", t.Host)
	}

	// TargetPortのバリデーション（共通関数使用）
	if err := port.ValidateRequiredPort(t.TargetPort, "target_port", t.Host); err != nil {
		return err
	}

	// ListenPortが指定されていない場合はTargetPortを使用
	if t.ListenPort == 0 {
		t.ListenPort = t.TargetPort
	}

	// 特権ポート警告
	port.WarnPrivilegedPort(t.ListenPort, "listen_port", t.Host)

	// .localhostドメイン警告（macOSで問題になる）
	port.WarnLocalhostTLD(t.Host, t.Host)

	return nil
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}

	// グローバルClusterのトリム
	cfg.Cluster = strings.TrimSpace(cfg.Cluster)

	// デフォルト値設定
	if cfg.ListenerPort == 0 {
		cfg.ListenerPort = 80
	} else if err := port.ValidatePort(cfg.ListenerPort, "listener_port", "config"); err != nil {
		return nil, err
	}

	// 特権ポート警告（デフォルト値80は除外）
	if cfg.ListenerPort != 80 {
		port.WarnPrivilegedPort(cfg.ListenerPort, "listener_port", "config")
	}

	if len(cfg.Services) == 0 {
		return nil, fmt.Errorf("no services configured in %s", path)
	}

	// TCPサービスが存在するかチェック
	hasTCPService := false
	for _, svcDef := range cfg.Services {
		svc := svcDef.Get()
		if _, ok := svc.(*TCPService); ok {
			hasTCPService = true
			break
		}
	}

	// SSHBastionのフィールドをトリムし、projectフォールバックを実施
	for _, bastion := range cfg.SSHBastions {
		bastion.Instance = strings.TrimSpace(bastion.Instance)
		bastion.Zone = strings.TrimSpace(bastion.Zone)
		bastion.Project = strings.TrimSpace(bastion.Project)
		bastion.SSHKeyPath = strings.TrimSpace(bastion.SSHKeyPath)
		bastion.SSHUser = strings.TrimSpace(bastion.SSHUser)

		// TCPサービスが存在し、projectが空の場合はフォールバック
		if hasTCPService && bastion.Project == "" {
			if project := os.Getenv("GOOGLE_CLOUD_PROJECT"); project != "" {
				bastion.Project = project
			} else if project := os.Getenv("GCLOUD_PROJECT"); project != "" {
				bastion.Project = project
			} else if project := os.Getenv("CLOUDSDK_CORE_PROJECT"); project != "" {
				bastion.Project = project
			}
		}
	}

	// バリデーション
	for i, svcDef := range cfg.Services {
		svc := svcDef.Get()
		if svc == nil {
			return nil, fmt.Errorf("invalid service entry at index %d: service is nil", i)
		}

		// 文字列フィールドのトリム（各サービス型で実施）
		trimServiceFields(svc)

		// 各サービスのバリデーション
		if err := svc.Validate(&cfg); err != nil {
			return nil, fmt.Errorf("invalid service entry at index %d: %w", i, err)
		}
	}

	// ポート競合チェック
	// 注意: TCPサービスはここではチェックしない
	// TCPサービスは実行時にloopback IPが割り当てられるため、
	// 同じポートでも異なるIPにバインドされ競合しない
	// （visitor.goでIP割り当て後にチェックする）
	checker := port.NewPortConflictChecker()
	port.RegisterPort(checker, cfg.ListenerPort, "listener_port")

	for _, svcDef := range cfg.Services {
		svc := svcDef.Get()
		switch s := svc.(type) {
		case *KubernetesService:
			if s.ListenerPort != 0 {
				port.RegisterPort(checker, s.ListenerPort, s.Host)
			}
			// TCPService は実行時にloopback IPが割り当てられてから
			// visitor.go でチェックするため、ここではスキップ
		}
	}

	return &cfg, nil
}

// trimServiceFields は文字列フィールドをトリム
func trimServiceFields(svc Service) {
	switch s := svc.(type) {
	case *KubernetesService:
		s.Host = strings.TrimSpace(s.Host)
		s.Namespace = strings.TrimSpace(s.Namespace)
		s.Service = strings.TrimSpace(s.Service)
		s.PortName = strings.TrimSpace(s.PortName)
		s.Protocol = strings.TrimSpace(s.Protocol)
		s.Cluster = strings.TrimSpace(s.Cluster)
	case *TCPService:
		s.Host = strings.TrimSpace(s.Host)
		s.SSHBastion = strings.TrimSpace(s.SSHBastion)
		s.TargetHost = strings.TrimSpace(s.TargetHost)
	}
}

type MockConfig struct {
	Mocks []MockService `yaml:"mocks"`
}

type MockService struct {
	Namespace    string           `yaml:"namespace"`
	Service      string           `yaml:"service"`
	PortName     string           `yaml:"port_name"`
	ResolvedPort port.ServicePort `yaml:"resolved_port"`
}

func LoadMockConfig(path string) (*MockConfig, error) {
	if path == "" {
		return nil, nil
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var mockCfg MockConfig
	if err := yaml.Unmarshal(b, &mockCfg); err != nil {
		return nil, err
	}

	return &mockCfg, nil
}
