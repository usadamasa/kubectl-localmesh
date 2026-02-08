package gcp

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// FindSSHAuthMethods は SSH認証メソッドを構築します:
//  1. configPath が空でなければ、そのパスの鍵ファイルを読み込み
//  2. ~/.ssh/google_compute_engine (gcloudデフォルト)
//  3. ~/.ssh/id_ed25519
//  4. ~/.ssh/id_rsa
//  5. SSH_AUTH_SOCK が設定されていればssh-agentを使用
//
// 見つかった鍵とssh-agentの両方を認証メソッドとして返します。
func FindSSHAuthMethods(configPath string) ([]ssh.AuthMethod, string, error) {
	var methods []ssh.AuthMethod
	var keySource string

	// 鍵ファイルの探索
	if configPath != "" {
		if signer, err := tryLoadKey(configPath); err == nil {
			methods = append(methods, ssh.PublicKeys(signer))
			keySource = configPath
		}
	}

	if keySource == "" {
		home := expandHome("~")
		keyPaths := []string{
			filepath.Join(home, ".ssh", "google_compute_engine"),
			filepath.Join(home, ".ssh", "id_ed25519"),
			filepath.Join(home, ".ssh", "id_rsa"),
		}
		for _, p := range keyPaths {
			if signer, err := tryLoadKey(p); err == nil {
				methods = append(methods, ssh.PublicKeys(signer))
				keySource = p
				break
			}
		}
	}

	// ssh-agentからの認証を追加
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			agentClient := agent.NewClient(conn)
			methods = append(methods, ssh.PublicKeysCallback(agentClient.Signers))
			if keySource == "" {
				keySource = "ssh-agent"
			} else {
				keySource += " + ssh-agent"
			}
		}
	}

	if len(methods) == 0 {
		return nil, "", fmt.Errorf(`SSH key not found. Searched paths:
  - ~/.ssh/google_compute_engine
  - ~/.ssh/id_ed25519
  - ~/.ssh/id_rsa

Also checked SSH_AUTH_SOCK for ssh-agent (not available).

To generate an SSH key for GCP, run:
  gcloud compute ssh INSTANCE_NAME --zone=ZONE --project=PROJECT --tunnel-through-iap

This will automatically create ~/.ssh/google_compute_engine

Or specify a custom key path in config:
  ssh_bastions:
    primary:
      ssh_key_path: /path/to/your/key`)
	}

	return methods, keySource, nil
}

// FindSSHKey は後方互換性のために残されています。
// 新しいコードは FindSSHAuthMethods を使用してください。
func FindSSHKey(configPath string) (ssh.Signer, error) {
	if configPath != "" {
		signer, err := tryLoadKey(configPath)
		if err == nil {
			return signer, nil
		}
	}

	home := expandHome("~")

	if signer, err := tryLoadKey(filepath.Join(home, ".ssh", "google_compute_engine")); err == nil {
		return signer, nil
	}

	if signer, err := tryLoadKey(filepath.Join(home, ".ssh", "id_ed25519")); err == nil {
		return signer, nil
	}

	if signer, err := tryLoadKey(filepath.Join(home, ".ssh", "id_rsa")); err == nil {
		return signer, nil
	}

	return nil, fmt.Errorf(`SSH key not found. Searched paths:
  - ~/.ssh/google_compute_engine
  - ~/.ssh/id_ed25519
  - ~/.ssh/id_rsa

To generate an SSH key for GCP, run:
  gcloud compute ssh INSTANCE_NAME --zone=ZONE --project=PROJECT --tunnel-through-iap`)
}

// ResolveSSHUser はSSH接続時のユーザー名を決定します:
//   - configUser が空でなければそのまま返す
//   - SUDO_USER（sudo実行時の元ユーザー）
//   - USER（macOS/Linux）
//   - USERNAME（Windows互換）
//   - すべて空なら "root" をフォールバック
func ResolveSSHUser(configUser string) string {
	if configUser != "" {
		return configUser
	}

	// sudo経由で実行された場合、元のユーザー名を使用
	// GCPのSSH鍵はsudo前のユーザー名で登録されているため
	if user := os.Getenv("SUDO_USER"); user != "" {
		return user
	}

	// macOS/Linux
	if user := os.Getenv("USER"); user != "" {
		return user
	}

	// Windows
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}

	// フォールバック
	return "root"
}

// tryLoadKey はファイルを読み込み、SSH秘密鍵をパースします。
// 証明書ファイル（path + "-cert.pub"）が存在すれば、証明書署名者として返します。
func tryLoadKey(path string) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, err
	}

	// 証明書ファイルが存在すれば証明書署名者を返す（OS Login環境用）
	certPath := path + "-cert.pub"
	certData, err := os.ReadFile(certPath)
	if err == nil {
		pubKey, _, _, _, parseErr := ssh.ParseAuthorizedKey(certData)
		if parseErr == nil {
			if cert, ok := pubKey.(*ssh.Certificate); ok {
				certSigner, certErr := ssh.NewCertSigner(cert, signer)
				if certErr == nil {
					return certSigner, nil
				}
			}
		}
	}

	return signer, nil
}

// expandHome は ~ をホームディレクトリに展開します。
func expandHome(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}

	if len(path) > 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}

	return path
}
