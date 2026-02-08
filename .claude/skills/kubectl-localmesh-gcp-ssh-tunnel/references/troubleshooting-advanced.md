# 高度なトラブルシューティング

GCP SSH Tunnelの認証、鍵管理、OS Login解決に関する詳細な診断手順。

> このドキュメントはSKILL.mdの「ユースケース別ワークフロー」で問題が解決しない場合に参照してください。

## GCP OS Login ユーザー名解決

### 解決フロー (`auth.go`)

```
1. config の ssh_user が指定されている?
   → YES: そのまま使用
   → NO: ↓

2. OS Login API でPOSIXユーザー名を取得:
   a. OAuth2 userinfo エンドポイントからメールアドレス取得
      GET https://www.googleapis.com/oauth2/v3/userinfo
   b. OS Login API でPOSIXアカウント取得
      GET https://oslogin.googleapis.com/v1/users/{email}/loginProfile
   → 成功: POSIXユーザー名を使用
   → 失敗: ↓

3. フォールバック: SUDO_USER → USER → USERNAME → "root"
```

### ユーザー名変換規則

OS Loginは Googleアカウントのメールアドレスを以下のルールでPOSIXユーザー名に変換する:

```
masaru.uchida@example.com → masaru_uchida_example_com
```

- `.` (ドット) → `_` (アンダースコア)
- `@` (アット) → `_` (アンダースコア)
- ドメイン部分の `.` も `_` に変換

### OS Login の既知の問題

| 問題 | 原因 | 対策 |
|------|------|------|
| サービスアカウントで認証失敗 | userinfoエンドポイントがメールを返さない | `ssh_user` で明示指定 |
| OS Login無効環境でAPI失敗 | OS Login APIが404を返す | ローカルユーザー名にフォールバック(自動) |
| gcloudと異なるユーザー名 | gcloudが独自のユーザー名で鍵を登録 | `ssh_user` で明示指定、または gcloud側に合わせる |

## SSH証明書と鍵管理 (`ssh_keys.go`)

### `-cert.pub` ファイル

gcloud は SSH鍵と同時に証明書ファイルを生成する場合がある:

```
~/.ssh/google_compute_engine           ← 秘密鍵
~/.ssh/google_compute_engine.pub       ← 公開鍵
~/.ssh/google_compute_engine-cert.pub  ← 証明書
```

`tryLoadKey` は秘密鍵ファイルと同じパスに `-cert.pub` があれば `ssh.NewCertSigner` で証明書署名者を返す。
証明書が期限切れの場合、通常の公開鍵認証にフォールバックする。

### SSH鍵の探索順序

`FindSSHAuthMethods` は以下の順序で鍵を探索する:

| 優先度 | パス/ソース | 条件 |
|--------|-----------|------|
| 1 | config の `ssh_key_path` | 明示指定時のみ |
| 2 | `~/.ssh/google_compute_engine` | gcloudデフォルト |
| 3 | `~/.ssh/id_ed25519` | 汎用Ed25519鍵 |
| 4 | `~/.ssh/id_rsa` | 汎用RSA鍵 |
| 5 | `SSH_AUTH_SOCK` 経由の ssh-agent | 環境変数が設定されている場合 |

- 鍵ファイルとssh-agentの**両方**が認証メソッドとして追加される
- 鍵ファイルは最初に見つかった1つのみ使用
- ssh-agentは鍵ファイルに追加で使用される

## デバッグチェックポイント

### 1. ADC認証の確認

```bash
# ADCが設定されているか
gcloud auth application-default print-access-token

# トークンのスコープ確認(メールアドレスが返ればOK)
curl -H "Authorization: Bearer $(gcloud auth application-default print-access-token)" \
  https://www.googleapis.com/oauth2/v3/userinfo
```

**よくある失敗:**
- `Could not automatically determine credentials`: ADC未設定 → `gcloud auth application-default login`
- メールが空: サービスアカウントの場合がある → `ssh_user` 明示指定を検討

### 2. IAP Tunnel接続の確認

```bash
# gcloud経由でIAP接続テスト
gcloud compute ssh INSTANCE --zone=ZONE --project=PROJECT --tunnel-through-iap -- echo OK
```

**よくある失敗:**
- `Permission denied`: IAP-secured Tunnel Userロールが必要
- `Connection timed out`: ファイアウォールルールでIAP IP範囲(35.235.240.0/20)を許可しているか確認

### 3. SSH認証の確認

```bash
# SSH鍵の存在確認
ls -la ~/.ssh/google_compute_engine ~/.ssh/id_ed25519 ~/.ssh/id_rsa 2>/dev/null

# 証明書ファイルの確認
ls -la ~/.ssh/*-cert.pub 2>/dev/null

# 証明書の有効期限確認
ssh-keygen -L -f ~/.ssh/google_compute_engine-cert.pub 2>/dev/null | grep Valid

# ssh-agent確認
echo $SSH_AUTH_SOCK
ssh-add -l
```

**よくある失敗:**
- 鍵ファイルが見つからない: `gcloud compute ssh` で初回接続すると自動生成される
- 証明書期限切れ: `gcloud compute ssh` で再接続すると証明書が更新される

### 4. OS Loginユーザー名の確認

```bash
# OS Login設定確認
gcloud compute project-info describe --project=PROJECT | grep enableOsLogin

# POSIXユーザー名確認
gcloud compute os-login describe-profile
```

**よくある失敗:**
- OS Loginが無効: メタデータで `enable-oslogin=TRUE` が設定されていない → ローカルユーザー名が使われる
- プロファイルにPOSIXアカウントがない: `gcloud compute ssh` で初回接続すると自動作成される

### 5. kubectl-localmesh デバッグモード

```bash
# デバッグログで詳細確認
sudo kubectl-localmesh --log-level debug up -f services.yaml --experimental-ssh
```

出力例:
```
SSH auth: user=masaru_uchida_example_com, key=~/.ssh/google_compute_engine + ssh-agent
SSH tunnel established: bastion-1 (user: masaru_uchida_example_com)
```

確認ポイント:
- `user=` が期待するユーザー名か
- `key=` が期待する鍵ファイルか
- `SSH tunnel established` が表示されるか

## 既知の問題と解決状況

| 問題 | 状況 | 対策 |
|------|------|------|
| OS Loginユーザー名解決の不安定 | 未解決 | `ssh_user` で明示指定 |
| SSH証明書の有効期限切れ | 未解決 | `gcloud compute ssh` で鍵を再生成 |
| ssh-agent転送時のsudo問題 | 未解決 | `SSH_AUTH_SOCK` がsudo後にリセットされる |
| gcloud CLI依存(デフォルト) | 設計通り | experimental実装で解消可能 |

### ssh-agent + sudo問題の詳細

kubectl-localmeshは`/etc/hosts`編集のためsudo実行が必要。
sudo実行時、`SSH_AUTH_SOCK`環境変数がリセットされるため、ssh-agentが利用できなくなる。

**回避策:**

```bash
# env_keepでSSH_AUTH_SOCKを保持(sudoersに追加)
# /etc/sudoers.d/ssh-agent
Defaults env_keep += "SSH_AUTH_SOCK"

# または実行時にSSH_AUTH_SOCKを明示的に渡す
sudo SSH_AUTH_SOCK=$SSH_AUTH_SOCK kubectl-localmesh up -f services.yaml --experimental-ssh
```

## 関連ファイル

| ファイル | 内容 |
|---------|------|
| `internal/gcp/auth.go` | ADC認証 + OS Login API |
| `internal/gcp/auth_test.go` | テスト |
| `internal/gcp/ssh_keys.go` | SSH鍵探索 + ssh-agent |
| `internal/gcp/ssh_keys_test.go` | テスト |
