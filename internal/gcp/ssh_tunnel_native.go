package gcp

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/usadamasa/kubectl-localmesh/internal/config"
	"github.com/usadamasa/kubectl-localmesh/internal/log"
	"github.com/usadamasa/kubectl-localmesh/internal/port"
)

// StartGCPSSHTunnelNative はGo SDKを使用してGCP Compute Instance経由でSSH tunnelを確立し、
// ローカルポートからターゲットホスト:ポートへのポートフォワーディングを行います。
// gcloud CLIに依存せず、IAP TCP Tunnel + SSH を純Goで実装しています。
// contextがキャンセルされるまで自動再接続を繰り返します。
//
// [experimental] --experimental-ssh フラグで有効化されます。
func StartGCPSSHTunnelNative(
	ctx context.Context,
	bastion *config.SSHBastion,
	localPort port.LocalPort,
	targetHost string,
	targetPort port.TCPPort,
	logger *log.Logger,
) error {
	// パラメータのバリデーション
	if bastion == nil {
		return fmt.Errorf("bastion is nil")
	}
	if bastion.Instance == "" {
		return fmt.Errorf("bastion instance name is empty")
	}
	if bastion.Zone == "" {
		return fmt.Errorf("bastion zone is empty")
	}
	if !port.IsValid(localPort) {
		return fmt.Errorf("invalid local port: %d", localPort)
	}
	if targetHost == "" {
		return fmt.Errorf("target host is empty")
	}
	if !port.IsValid(targetPort) {
		return fmt.Errorf("invalid target port: %d", targetPort)
	}

	// 初回接続を試行（失敗時は即座にエラーを返す）
	err := startSingleSSHTunnelNative(ctx, bastion, localPort, targetHost, targetPort, logger)
	if ctx.Err() != nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("SSH tunnel to %s failed: %w", bastion.Instance, err)
	}

	// 自動再接続ループ（k8s port-forwardと同様）
	for {
		// 300ms待機後に再接続
		time.Sleep(300 * time.Millisecond)

		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// SSH tunnel確立を試行
		err := startSingleSSHTunnelNative(ctx, bastion, localPort, targetHost, targetPort, logger)

		// contextキャンセル時は正常終了
		if ctx.Err() != nil {
			return nil
		}

		if err != nil {
			logger.Infof("[WARN] SSH tunnel disconnected: %s -> %s:%d (reconnecting...): %v",
				bastion.Instance, targetHost, int(targetPort), err)
		}
	}
}

// startSingleSSHTunnelNative は1回のSSH tunnel接続を試行します。
// IAP TCP Tunnel → SSH → ポートフォワーディングの3段パイプラインで実装。
// 接続が切断されるか、エラーが発生した場合に返ります。
func startSingleSSHTunnelNative(
	ctx context.Context,
	bastion *config.SSHBastion,
	localPort port.LocalPort,
	targetHost string,
	targetPort port.TCPPort,
	logger *log.Logger,
) error {
	// 1. ADC認証トークンを取得
	ts, err := TokenSource(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token source: %w", err)
	}

	// 2. IAP Tunnel経由でbastion port 22にnet.Connを確立
	dialer := &DefaultIAPDialer{}
	iapConn, err := dialer.DialContext(ctx, bastion.Project, bastion.Zone, bastion.Instance, 22, ts)
	if err != nil {
		return fmt.Errorf("failed to establish IAP tunnel to %s: %w", bastion.Instance, err)
	}
	defer func() { _ = iapConn.Close() }()

	// 4. SSH認証メソッドを構築
	authMethods, keySource, err := FindSSHAuthMethods(bastion.SSHKeyPath)
	if err != nil {
		return err
	}

	// 5. SSHユーザー名を解決
	// configで明示指定されていない場合、OS Login APIでPOSIXユーザー名を取得
	var sshUser string
	if bastion.SSHUser != "" {
		sshUser = bastion.SSHUser
	} else if osLoginUser, osErr := ResolveOSLoginUser(ctx, ts); osErr == nil {
		sshUser = osLoginUser
	} else {
		logger.Debugf("OS Login resolution failed (falling back to local user): %v", osErr)
		sshUser = ResolveSSHUser("")
	}

	logger.Debugf("SSH auth: user=%s, key=%s", sshUser, keySource)

	// 6. SSH接続を確立
	sshConn, chans, reqs, err := ssh.NewClientConn(iapConn, bastion.Instance, &ssh.ClientConfig{
		User:            sshUser,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // ローカルdev用
		Timeout:         30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to establish SSH connection to %s (user: %s, key: %s): %w",
			bastion.Instance, sshUser, keySource, err)
	}
	defer func() { _ = sshConn.Close() }()

	sshClient := ssh.NewClient(sshConn, chans, reqs)
	defer func() { _ = sshClient.Close() }()

	logger.Debugf("SSH tunnel established: %s (user: %s)", bastion.Instance, sshUser)

	// 7. ローカルポートでリッスン
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", int(localPort)))
	if err != nil {
		return fmt.Errorf("failed to listen on 127.0.0.1:%d: %w", int(localPort), err)
	}
	defer func() { _ = listener.Close() }()

	// contextキャンセル時にリスナーを閉じる
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	// 8. accept loop: 各接続をSSHチャネル経由でtargetHost:targetPortに転送
	for {
		localConn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("failed to accept connection: %w", err)
		}

		// SSHチャネル経由でリモートホストに接続
		remoteAddr := fmt.Sprintf("%s:%d", targetHost, int(targetPort))
		remoteConn, err := sshClient.Dial("tcp", remoteAddr)
		if err != nil {
			_ = localConn.Close()
			logger.Debugf("failed to dial remote %s via SSH: %v", remoteAddr, err)
			continue
		}

		// 双方向コピー
		go forwardConnection(localConn, remoteConn)
	}
}

// forwardConnection は2つのnet.Conn間で双方向にデータをコピーします。
// どちらか一方が閉じるか、エラーが発生した場合に両方を閉じます。
func forwardConnection(local, remote net.Conn) {
	done := make(chan struct{}, 2)

	go func() {
		_, _ = io.Copy(remote, local)
		done <- struct{}{}
	}()

	go func() {
		_, _ = io.Copy(local, remote)
		done <- struct{}{}
	}()

	// 最初の完了を待つ
	<-done

	// 両方を閉じる
	_ = local.Close()
	_ = remote.Close()
}
