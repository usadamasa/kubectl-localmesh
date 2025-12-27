package run

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/usadamasa/kubectl-local-mesh/internal/config"
	"github.com/usadamasa/kubectl-local-mesh/internal/envoy"
	"github.com/usadamasa/kubectl-local-mesh/internal/hosts"
	"github.com/usadamasa/kubectl-local-mesh/internal/kube"
	"github.com/usadamasa/kubectl-local-mesh/internal/pf"
)

func Run(ctx context.Context, cfg *config.Config, logLevel string, updateHosts bool) error {
	// /etc/hosts更新が必要な場合
	if updateHosts {
		// 権限チェック
		if !hosts.HasPermission() {
			return fmt.Errorf("need sudo: try 'sudo kubectl-local-mesh ...'")
		}

		// ホスト名リストを収集
		var hostnames []string
		for _, s := range cfg.Services {
			hostnames = append(hostnames, s.Host)
		}

		// /etc/hostsに追加
		if err := hosts.AddEntries(hostnames); err != nil {
			return fmt.Errorf("failed to update /etc/hosts: %w", err)
		}
		fmt.Println("/etc/hosts updated successfully")

		// 終了時にクリーンアップ
		defer func() {
			if err := hosts.RemoveEntries(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to clean up /etc/hosts: %v\n", err)
			} else {
				fmt.Println("/etc/hosts cleaned up")
			}
		}()
	}

	tmpDir, err := os.MkdirTemp("", "kubectl-local-mesh-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	var routes []envoy.Route
	var procs []*exec.Cmd

	for _, s := range cfg.Services {
		remotePort, err := kube.ResolveServicePort(
			ctx,
			s.Namespace,
			s.Service,
			s.PortName,
			s.Port,
		)
		if err != nil {
			return err
		}

		localPort, err := pf.FreeLocalPort()
		if err != nil {
			return err
		}

		clusterName := sanitize(fmt.Sprintf("%s_%s_%d", s.Namespace, s.Service, remotePort))

		fmt.Printf(
			"pf: %-30s -> %s/%s:%d via 127.0.0.1:%d\n",
			s.Host,
			s.Namespace,
			s.Service,
			remotePort,
			localPort,
		)

		cmd, err := pf.StartPortForwardLoop(
			ctx,
			s.Namespace,
			s.Service,
			localPort,
			remotePort,
		)
		if err != nil {
			return err
		}
		procs = append(procs, cmd)

		routes = append(routes, envoy.Route{
			Host:        s.Host,
			LocalPort:   localPort,
			ClusterName: clusterName,
		})
	}

	envoyCfg := envoy.BuildConfig(cfg.ListenerPort, routes)
	envoyPath := filepath.Join(tmpDir, "envoy.yaml")

	b, err := yaml.Marshal(envoyCfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(envoyPath, b, 0644); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("envoy config: %s\n", envoyPath)
	fmt.Printf("listen: 0.0.0.0:%d\n\n", cfg.ListenerPort)

	envoyCmd := exec.CommandContext(
		ctx,
		"envoy",
		"-c", envoyPath,
		"-l", logLevel,
	)
	envoyCmd.Stdout = os.Stdout
	envoyCmd.Stderr = os.Stderr

	err = envoyCmd.Run()

	for _, p := range procs {
		pf.Terminate(p)
	}
	return err
}

func DumpEnvoyConfig(ctx context.Context, cfg *config.Config, mockConfigPath string) error {
	var mockCfg *config.MockConfig
	var err error

	// モック設定の読み込み
	if mockConfigPath != "" {
		mockCfg, err = config.LoadMockConfig(mockConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load mock config: %w", err)
		}
	}

	var routes []envoy.Route

	for i, s := range cfg.Services {
		var remotePort int

		// モック設定が指定されている場合はモックから取得
		if mockCfg != nil {
			remotePort, err = findMockPort(mockCfg, s.Namespace, s.Service, s.PortName)
			if err != nil {
				return err
			}
		} else {
			// モック設定がない場合は通常通りkubectlで解決
			remotePort, err = kube.ResolveServicePort(
				ctx,
				s.Namespace,
				s.Service,
				s.PortName,
				s.Port,
			)
			if err != nil {
				return err
			}
		}

		// ダミーのローカルポートを割り当て（実際にはport-forwardしない）
		dummyLocalPort := 10000 + i

		clusterName := sanitize(fmt.Sprintf("%s_%s_%d", s.Namespace, s.Service, remotePort))

		routes = append(routes, envoy.Route{
			Host:        s.Host,
			LocalPort:   dummyLocalPort,
			ClusterName: clusterName,
		})
	}

	envoyCfg := envoy.BuildConfig(cfg.ListenerPort, routes)

	b, err := yaml.Marshal(envoyCfg)
	if err != nil {
		return err
	}

	fmt.Print(string(b))
	return nil
}

func findMockPort(mockCfg *config.MockConfig, namespace, service, portName string) (int, error) {
	for _, m := range mockCfg.Mocks {
		if m.Namespace == namespace && m.Service == service && m.PortName == portName {
			return m.ResolvedPort, nil
		}
	}
	return 0, fmt.Errorf("mock config not found for %s/%s (port_name=%s)", namespace, service, portName)
}

func sanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 'a' && r <= 'z' ||
			r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' ||
			r == '_' {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}
