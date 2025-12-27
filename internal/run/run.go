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
	"github.com/usadamasa/kubectl-local-mesh/internal/kube"
	"github.com/usadamasa/kubectl-local-mesh/internal/pf"
)

func Run(ctx context.Context, cfg *config.Config, logLevel string) error {
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

func DumpEnvoyConfig(ctx context.Context, cfg *config.Config) error {
	var routes []envoy.Route

	for i, s := range cfg.Services {
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
