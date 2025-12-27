package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenerPort int       `yaml:"listener_port"`
	Services     []Service `yaml:"services"`
}

type Service struct {
	Host      string `yaml:"host"`
	Namespace string `yaml:"namespace"`
	Service   string `yaml:"service"`
	PortName  string `yaml:"port_name"`
	Port      int    `yaml:"port"`
	Type      string `yaml:"type"` // http|grpc (今は主にメタ)
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

	if cfg.ListenerPort == 0 {
		cfg.ListenerPort = 80
	}
	if len(cfg.Services) == 0 {
		return nil, fmt.Errorf("no services configured in %s", path)
	}

	for i := range cfg.Services {
		s := &cfg.Services[i]
		s.Host = strings.TrimSpace(s.Host)
		s.Namespace = strings.TrimSpace(s.Namespace)
		s.Service = strings.TrimSpace(s.Service)
		s.PortName = strings.TrimSpace(s.PortName)
		s.Type = strings.TrimSpace(s.Type)

		if s.Host == "" || s.Namespace == "" || s.Service == "" {
			return nil, fmt.Errorf("invalid service entry at index %d: host/namespace/service are required", i)
		}
		if s.Port == 0 && s.PortName == "" {
			// OK: ports[0] fallbackを使う
		}
	}

	return &cfg, nil
}

type MockConfig struct {
	Mocks []MockService `yaml:"mocks"`
}

type MockService struct {
	Namespace    string `yaml:"namespace"`
	Service      string `yaml:"service"`
	PortName     string `yaml:"port_name"`
	ResolvedPort int    `yaml:"resolved_port"`
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
