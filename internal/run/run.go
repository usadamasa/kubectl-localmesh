package run

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/usadamasa/kubectl-localmesh/internal/config"
	"github.com/usadamasa/kubectl-localmesh/internal/envoy"
	"github.com/usadamasa/kubectl-localmesh/internal/hosts"
	"github.com/usadamasa/kubectl-localmesh/internal/log"
	"github.com/usadamasa/kubectl-localmesh/internal/loopback"
	"gopkg.in/yaml.v3"
)

func Run(ctx context.Context, cfg *config.Config, logLevel string, updateHosts bool, experimentalSSH bool) error {
	// Logger初期化
	logger := log.New(logLevel)

	tmpDir, err := os.MkdirTemp("", "kubectl-localmesh-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Visitor の生成（Kubernetes clientはサービスごとにlazy初期化）
	visitor := NewRunVisitor(ctx, cfg, logger, experimentalSSH)

	// Visitorパターンで各サービスを処理
	for _, svcDef := range cfg.Services {
		svc := svcDef.Get()
		if err := svc.Accept(visitor); err != nil {
			return err
		}
	}

	// loopback IPエイリアス追加（TCPサービス用）
	// AliasManagerを先に作成し、deferを先に設定することで
	// AddAlias途中で失敗しても追加成功した分だけ確実に削除する
	aliasMgr := loopback.NewAliasManager()
	defer func() {
		added := aliasMgr.GetAdded()
		aliasMgr.RemoveAdded()
		if len(added) > 0 {
			logger.Debug("loopback aliases removed")
		}
	}()

	aliases := visitor.GetIPAllocator().GetAliases()
	for _, ip := range aliases {
		if err := aliasMgr.AddAlias(ip); err != nil {
			return fmt.Errorf("failed to add loopback alias %s: %w", ip, err)
		}
	}
	if len(aliases) > 0 {
		logger.Debugf("loopback aliases added: %v", aliases)
	}

	// /etc/hosts更新が必要な場合
	if updateHosts {
		// 権限チェック
		if !hosts.HasPermission() {
			return fmt.Errorf("need sudo: try 'sudo kubectl-localmesh ...'")
		}

		// ホストエントリを収集（TCPサービスは割り当てられたIPを使用）
		var entries []hosts.HostEntry
		for _, sc := range visitor.GetServiceConfigs() {
			switch b := sc.Builder.(type) {
			case *envoy.TCPServiceBuilder:
				entries = append(entries, hosts.HostEntry{
					Hostname: b.GetHost(),
					IP:       b.GetListenAddr(),
				})
			case *envoy.KubernetesServiceBuilder:
				entries = append(entries, hosts.HostEntry{
					Hostname: b.GetHost(),
					IP:       "127.0.0.1",
				})
			}
		}

		// /etc/hostsに追加
		if err := hosts.AddEntriesWithIPs(entries); err != nil {
			return fmt.Errorf("failed to update /etc/hosts: %w", err)
		}
		logger.Debug("/etc/hosts updated successfully")

		// 終了時にクリーンアップ
		defer func() {
			if err := hosts.RemoveEntries(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to clean up /etc/hosts: %v\n", err)
			} else {
				logger.Debug("/etc/hosts cleaned up")
			}
		}()
	}

	// Envoy設定生成
	envoyCfg := envoy.BuildConfig(cfg.ListenerPort, visitor.GetServiceConfigs())
	envoyPath := filepath.Join(tmpDir, "envoy.yaml")

	b, err := yaml.Marshal(envoyCfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(envoyPath, b, 0644); err != nil {
		return err
	}

	logger.Debugf("envoy config: %s", envoyPath)
	logger.Debugf("listen: 0.0.0.0:%d", cfg.ListenerPort)

	// サマリー出力
	summary := log.GenerateSummary(visitor.GetServiceSummaries(), cfg.ListenerPort)
	logger.Info(summary)

	envoyCmd := exec.CommandContext(
		ctx,
		"envoy",
		"-c", envoyPath,
		"-l", logger.EnvoyLevel(),
	)
	envoyCmd.Stdout = os.Stdout
	envoyCmd.Stderr = os.Stderr

	// Envoy実行（contextキャンセル時に自動終了）
	// port-forwardのgoroutineもcontextキャンセル時に自動終了する
	return envoyCmd.Run()
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
