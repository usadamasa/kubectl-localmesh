package run

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"k8s.io/client-go/kubernetes"

	"github.com/usadamasa/kubectl-localmesh/internal/config"
	"github.com/usadamasa/kubectl-localmesh/internal/envoy"
	"github.com/usadamasa/kubectl-localmesh/internal/hosts"
	"github.com/usadamasa/kubectl-localmesh/internal/k8s"
	"github.com/usadamasa/kubectl-localmesh/internal/pf"
)

func Run(ctx context.Context, cfg *config.Config, logLevel string, updateHosts bool) error {
	// Kubernetes client初期化
	clientset, restConfig, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// /etc/hosts更新が必要な場合
	if updateHosts {
		// 権限チェック
		if !hosts.HasPermission() {
			return fmt.Errorf("need sudo: try 'sudo kubectl-localmesh ...'")
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

	tmpDir, err := os.MkdirTemp("", "kubectl-localmesh-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	var routes []envoy.Route

	for _, s := range cfg.Services {
		remotePort, err := k8s.ResolveServicePort(
			ctx,
			clientset,
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

		// port-forwardをgoroutineで起動（自動再接続）
		go func(ns, svc string, local, remote int) {
			if err := k8s.StartPortForwardLoop(
				ctx,
				restConfig,
				clientset,
				ns,
				svc,
				local,
				remote,
			); err != nil {
				// contextキャンセル以外のエラーをログ出力
				if ctx.Err() == nil {
					fmt.Fprintf(os.Stderr, "port-forward error for %s/%s: %v\n", ns, svc, err)
				}
			}
		}(s.Namespace, s.Service, localPort, remotePort)

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

	// Envoy実行（contextキャンセル時に自動終了）
	// port-forwardのgoroutineもcontextキャンセル時に自動終了する
	return envoyCmd.Run()
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

	// モックモードでない場合はKubernetes clientを初期化
	var clientset *kubernetes.Clientset
	if mockCfg == nil {
		var k8sErr error
		clientset, _, k8sErr = k8s.NewClient()
		if k8sErr != nil {
			return fmt.Errorf("failed to create kubernetes client: %w", k8sErr)
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
			// モック設定がない場合は通常通りclient-goで解決
			remotePort, err = k8s.ResolveServicePort(
				ctx,
				clientset,
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
