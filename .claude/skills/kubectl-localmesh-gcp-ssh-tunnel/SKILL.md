---
name: kubectl-localmesh-gcp-ssh-tunnel
description: GCP SSH tunnel (IAP TCP Tunnel + SSH) の実装詳細、デバッグ方法、既知の問題を提供します
allowed-tools: ["Bash", "Read"]
---

# GCP SSH Tunnel スキル

kubectl-localmeshにおけるGCP SSH Bastionを経由したDB接続(TCP proxy)のSSH tunnel実装に関する知見。
DB(Cloud SQL等のPrivate IP)へのローカルアクセスを実現するための設計・運用・トラブルシューティングを提供する。

## 2つの実装方式

| | gcloud CLI (デフォルト) | Go SDK (experimental) |
|---|---|---|
| **ファイル** | `ssh_tunnel.go` | `ssh_tunnel_native.go` |
| **有効化** | デフォルト | `--experimental-ssh` フラグ |
| **依存** | `gcloud` CLI必須 | 単一バイナリで完結 |
| **認証管理** | gcloudが全自動 | ADC + OS Login API + SSH鍵探索 |
| **SSH証明書** | 自動管理 | `-cert.pub` ファイルを自動検出 |
| **安定性** | 安定した動作実績 | experimental |

### 選択フロー

```
gcloud CLI がインストールされている?
  → YES: デフォルト方式を使用(推奨)
  → NO: --experimental-ssh で Go SDK方式を使用
```

### gcloud CLI方式のコマンド

```
gcloud compute ssh INSTANCE \
  --project=PROJECT --zone=ZONE \
  --tunnel-through-iap \
  -- -L LOCAL_PORT:TARGET_HOST:TARGET_PORT -N \
  -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=30 \
  -o ServerAliveCountMax=3
```

## 実装アーキテクチャ (Go SDK方式)

### 3段パイプライン

Go SDK方式は以下の3段パイプラインで構成される:

```
ローカルポート (net.Listen on 127.0.0.1:LOCAL_PORT)
  ↓ accept loop (各TCP接続を個別に処理)
IAP TCP Tunnel (WebSocket → iapTunnelConn → net.Conn)
  ↓ iap_tunnel.go: バイナリフレーミング(DATA/ACK)
SSH接続 (golang.org/x/crypto/ssh)
  ↓ ssh_keys.go(鍵探索) + auth.go(OS Login解決)
リモートホスト (sshClient.Dial("tcp", TARGET_HOST:TARGET_PORT))
  ↓ 双方向 io.Copy
DB (Cloud SQL等)
```

### accept loop

`startSingleSSHTunnelNative` は:

1. ADC認証トークンを取得
2. IAP Tunnel経由でbastion port 22に`net.Conn`を確立
3. SSH認証メソッドを構築(鍵ファイル + ssh-agent)
4. SSHユーザー名を解決(OS Login API → フォールバック)
5. SSH接続を確立
6. `net.Listen`でローカルポートをリッスン
7. 各接続に対して`sshClient.Dial` + 双方向`io.Copy`

### 双方向転送 (`forwardConnection`)

```go
done := make(chan struct{}, 2)  // capacity 2: 両goroutineが同時完了してもブロックしない
go func() { io.Copy(remote, local); done <- struct{}{} }()
go func() { io.Copy(local, remote); done <- struct{}{} }()
<-done  // 最初の完了を待つ
// 両方をClose → もう一方のio.Copyも終了
```

### 300ms再接続ループ

```
初回接続 → 失敗したら即座にエラー返却
         → 成功 → 切断されるまでaccept loop
         → 300ms待機 → 再接続(無限ループ)
         → contextキャンセルで正常終了
```

k8s port-forwardの再接続(`internal/pf/forward.go`)と同じ300ms間隔を採用。
これはネットワーク一時障害からの復旧に十分な間隔でありつつ、
CPU負荷を抑えるバランスポイント。

## ユースケース別ワークフロー

### ケース1: 初期接続で失敗する

**症状:** `SSH tunnel to bastion-1 failed: ...` で即座にエラー

```
1. ADC認証を確認
   → gcloud auth application-default print-access-token

2. IAP Tunnel接続を確認
   → gcloud compute ssh INSTANCE --zone=ZONE --project=PROJECT --tunnel-through-iap -- echo OK

3. SSH認証を確認
   → ls -la ~/.ssh/google_compute_engine ~/.ssh/id_ed25519 ~/.ssh/id_rsa

4. デバッグログで詳細確認
   → sudo kubectl-localmesh --log-level debug up -f services.yaml [--experimental-ssh]
```

### ケース2: 接続後に頻繁に切断・再接続される

**症状:** `SSH tunnel disconnected: ... (reconnecting...)` が繰り返し表示される

```
1. Bastion Instanceの状態確認
   → gcloud compute instances describe INSTANCE --zone=ZONE --project=PROJECT

2. ネットワーク安定性の確認(IAP経由)
   → gcloud compute ssh INSTANCE --zone=ZONE --project=PROJECT --tunnel-through-iap -- uptime

3. ServerAliveInterval確認(gcloud方式のみ)
   → デフォルトで30秒間隔のkeepaliveが設定されている
```

### ケース3: SSH認証失敗 (experimental)

**症状:** `failed to establish SSH connection ... (user: xxx, key: yyy)` エラー

```
1. ユーザー名の確認
   → gcloud compute os-login describe-profile

2. SSH鍵の確認
   → ssh-keygen -L -f ~/.ssh/google_compute_engine-cert.pub  (証明書の有効期限)

3. 対策: config で ssh_user を明示指定
   ssh_bastions:
     primary:
       ssh_user: masaru_uchida_example_com
```

### ケース4: SSH証明書の期限切れ (experimental)

**症状:** 証明書認証失敗後にパブリックキー認証にフォールバックするが、それも失敗

```
1. 証明書の有効期限確認
   → ssh-keygen -L -f ~/.ssh/google_compute_engine-cert.pub | grep Valid

2. 証明書の再生成
   → gcloud compute ssh INSTANCE --zone=ZONE --project=PROJECT --tunnel-through-iap
   (接続するだけで証明書が自動更新される)
```

## クイック診断

診断スクリプトで環境を一括チェックできる:

```bash
bash .claude/skills/kubectl-localmesh-gcp-ssh-tunnel/scripts/diagnose-ssh-tunnel.sh
```

チェック項目:
- gcloud CLI の存在とアクティブアカウント
- ADC認証トークン取得テスト
- SSH鍵ファイル探索(google_compute_engine, id_ed25519, id_rsa)
- SSH証明書(-cert.pub)の有効期限
- ssh-agent接続確認
- OS Login POSIXユーザー名表示

出力例:
```
✅ Google Cloud SDK 400.0.0
✅ ADCトークン取得成功
✅ ~/.ssh/google_compute_engine
   📜 証明書: ~/.ssh/google_compute_engine-cert.pub
✅ SSH_AUTH_SOCK=/tmp/ssh-xxx/agent.123
✅ POSIXユーザー名: masaru_uchida_example_com
```

## よくあるエラー

| エラー症状 | 原因 | 解決策 |
|-----------|------|--------|
| `failed to get default token source` | ADC未設定 | `gcloud auth application-default login` |
| `failed to connect to IAP tunnel` | IAP権限不足 | IAP-secured Tunnel Userロール付与 |
| `SSH key not found` | SSH鍵がない | `gcloud compute ssh`で初回接続(鍵自動生成) |
| `failed to establish SSH connection` | ユーザー名/鍵の不一致 | `ssh_user`で明示指定 |
| `failed to listen on 127.0.0.1:PORT` | ポート使用中 | 他プロセスのポート使用を確認 |
| `Connection timed out` (IAP) | FW未設定 | 35.235.240.0/20 を許可 |

## 実装詳細リファレンス

より深い技術詳細が必要な場合は以下を参照:

| リファレンス | 内容 | 参照タイミング |
|------------|------|--------------|
| [IAP Tunnel Protocol](references/iap-tunnel-protocol.md) | バイナリフレーミング仕様、net.Conn実装、mutex設計 | iap_tunnel.goの実装を修正・理解する場合 |
| [Advanced Troubleshooting](references/troubleshooting-advanced.md) | OS Login解決フロー、SSH鍵管理、デバッグコマンド集、既知の問題 | ユースケース別ワークフローで解決しない場合 |

## 関連ファイル

| ファイル | 内容 |
|---------|------|
| `internal/gcp/ssh_tunnel.go` | gcloud CLI版(デフォルト) |
| `internal/gcp/ssh_tunnel_native.go` | Go SDK版(experimental) |
| `internal/gcp/auth.go` | ADC認証 + OS Login API |
| `internal/gcp/iap_tunnel.go` | IAP TCP Tunnel v4実装 |
| `internal/gcp/ssh_keys.go` | SSH鍵探索 + ssh-agent |
| `internal/run/visitor.go` | フラグによる呼び分け |
| `cmd/up.go` | `--experimental-ssh` フラグ定義 |

## 関連Skills

| スキル | 関連ポイント |
|--------|------------|
| [kubectl-localmesh-operations](../kubectl-localmesh-operations/SKILL.md) | 起動・停止の運用フロー、依存関係チェック |
| [kubectl-envoy-debugging](../kubectl-envoy-debugging/SKILL.md) | TCP proxyを含むEnvoy設定のデバッグ |
| [go-taskfile-workflow](../go-taskfile-workflow/SKILL.md) | ビルド・テスト実行 |
| [kubectl-localmesh-macos-localhost](../kubectl-localmesh-macos-localhost/SKILL.md) | TCPサービスの.localhostドメイン問題 |
