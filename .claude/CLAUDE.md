# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

`kubectl-localmesh`は、`kubectl port-forward`およびGCP SSH bastionをベースにしたローカル専用の疑似サービスメッシュツールです。複数のKubernetesサービス（HTTP/gRPC）およびGCP SSH bastion経由のDB接続（TCP）に対して、ローカルエントリポイントからアクセスできます。

**CLIフレームワーク:** [Cobra](https://github.com/spf13/cobra)を使用したサブコマンド構造

**重要な設計原則:**
- クラスタ側には何もインストールしない（kubectl primitives only）
- GCP SSH bastionを使用したDB接続（TCP proxy）のサポート
- ローカル開発・デバッグ専用（本番環境は対象外）
- 障害モードを明確に保つ
- 起動・破棄が容易

## アーキテクチャ

### Kubernetesサービス（HTTP/gRPC）

```
クライアント
  ↓ (http://users-api.localhost:80)
ローカルEnvoy (ホストベースルーティング)
  ↓ (動的に割り当てられたローカルポート)
kubectl port-forward (各サービスごとに自動起動・再接続)
  ↓
Kubernetesサービス
```

### DB接続（TCP via GCP SSH Bastion）

```
クライアント
  ↓ (tcp://db.localhost:5432)
ローカルEnvoy (TCP proxy)
  ↓ (動的に割り当てられたローカルポート)
GCP SSH tunnel (GCP Compute Instance経由)
  ↓ (SSH port-forward)
GCP Bastion
  ↓
DB (Cloud SQL等、Private IP)

### 主要コンポーネント

1. **cmd** (`cmd/`)
   - Cobraベースのサブコマンド実装
   - `root.go`: ルートコマンド定義（グローバルフラグ: `--log-level`）
   - `up.go`: サービスメッシュ起動（upサブコマンド）
   - `validate.go`: 設定ファイルの検証（validateサブコマンド）
   - `dump_envoy_config.go`: Envoy設定のダンプ（dump-envoy-configサブコマンド）
   - (将来) `down.go`: サービスメッシュ停止
   - (将来) `status.go`: ステータス表示

2. **config** (`internal/config/`)
   - YAMLベースの設定ファイル読み込み
   - `listener_port`: Envoyが待ち受けるローカルポート（デフォルト: 80、HTTP/gRPC用）
   - `ssh_bastions`: GCP SSH bastion定義（instance, zone, project）
   - `services`: ルーティング対象のサービス一覧
     - Kubernetes: host, namespace, service, port/port_name, type (http|grpc)
     - DB via SSH: host, type (tcp), ssh_bastion, target_host, target_port

3. **kube** (`internal/k8s/`)
   - Kubernetes client-go wrapper
   - サービスのポート解決（port_name指定またはports[0]をフォールバック）
   - WebSocket-based port-forward実装

4. **pf** (`internal/pf/`)
   - ローカルポート割り当て（`FreeLocalPort`）

5. **gcp** (`internal/gcp/`)
   - **GCP SSH tunnel実装（デフォルト: `gcloud compute ssh`経由）**
   - `ssh_tunnel.go`: `gcloud compute ssh`ベースのSSH tunnel（デフォルト）
   - `ssh_tunnel_native.go`: **[experimental]** 純Go実装のSSH tunnel（`--experimental-ssh`で有効化）
   - `auth.go`: ADC認証トークン取得（OAuth2 TokenSource、experimental用）
   - `iap_tunnel.go`: IAP TCP Tunnel v4プロトコル実装（WebSocket経由、experimental用）
   - `ssh_keys.go`: SSH認証メソッドの構築（鍵ファイル探索 + ssh-agent対応、experimental用）
   - 自動再接続ループ（300ms間隔）

6. **envoy** (`internal/envoy/`)
   - Envoy設定ファイル（YAML）の動的生成
   - HTTP/2対応（h2c plaintext for gRPC）
   - ホストベースのvirtual_hosts設定（HTTP/gRPC）
   - **TCP proxy設定** (新規)
     - 各TCPサービス（DB）に独立したリスナー
     - 専用ポート番号（target_portで指定）

7. **hosts** (`internal/hosts/`)
   - /etc/hosts ファイルの管理
   - マーカーコメントによるエントリの追跡・削除
   - 書き込み権限チェック

8. **run** (`internal/run/`)
   - オーケストレーションロジック
   - 各サービスに対してport-forwardまたはSSH tunnelを起動
   - Envoy設定を生成・適用（HTTP/gRPC + TCP）
   - Envoyプロセスの起動・監視
   - クリーンアップ処理

9. **log** (`internal/log/`)
   - ログレベル階層化（warn/info/debug）
   - ユーザーフレンドリーなサマリー出力
   - `Logger`型によるログ出力の抽象化

10. **schemas** (`schemas/`)
    - 設定ファイルのJSON Schema定義（Draft 2020-12）
    - `config.schema.json`: 設定ファイルの構造定義
    - `embed.go`: `//go:embed`によるスキーマファイルのバイナリ埋め込み
    - `oneOf` + `enum`パターンでタグ付きユニオン（kind: kubernetes / tcp）を表現
    - `additionalProperties: false`で未知フィールド（タイポ）を検出
    - エディタ統合（yaml-language-server）にも使用

11. **validate** (`internal/validate/`)
    - JSON Schemaに基づく設定ファイル検証ロジック
    - `santhosh-tekuri/jsonschema/v6`ライブラリを使用
    - YAMLをmap[string]anyにアンマーシャルし、JSON互換型に変換後スキーマ検証
    - `ValidationResult`型で検証結果を構造化して返却
    - `validateCmd --strict`フラグから呼び出される

12. **loopback** (`internal/loopback/`)
    - **macOS限定機能**
    - loopback IPエイリアス管理（TCPサービス用）
    - `IPAllocator`: 127.0.0.2〜127.0.0.254から順次IPを割り当て
    - `AliasManager`: `ifconfig lo0 alias`による追加・削除
    - 複数TCPサービスが同じポート（例: 5432）を使用する場合の重複回避
    - 起動時にエイリアス追加、終了時にdeferで自動削除

## 開発ワークフロー

このプロジェクトでは、開発タスクの実行に[Task](https://taskfile.dev)を使用します。
詳細な開発ワークフローについては、以下のskillsを参照してください。

### 利用可能なSkills

開発作業には、以下のskillsが利用できます：

#### `go-taskfile-workflow` - ビルド・テスト・品質管理
Taskfileを使った標準開発ワークフローを提供します。

**主な機能**:
- `task build`: プロジェクトビルド
- `task test`: テスト実行
- `task lint`: 静的解析（yamllint + golangci-lint）
- `task format`: コードフォーマット
- `aqua install`: 開発ツールのインストール

詳細: `.claude/skills/go-taskfile-workflow/SKILL.md`

#### `kubectl-envoy-debugging` - デバッグ・設定確認
Envoy設定の確認とデバッグを支援します。

**主な機能**:
- `dump-envoy-config`: Envoy設定のダンプ（サブコマンド）
- `--mock-config`: オフラインモード（クラスタ接続不要、dump-envoy-configのオプション）
- `--log-level debug`: 詳細デバッグログ（グローバルフラグ）
- Envoy設定の検証とトラブルシューティング

詳細: `.claude/skills/kubectl-envoy-debugging/SKILL.md`

#### `kubectl-localmesh-operations` - 起動・運用
kubectl-localmesh固有の運用操作を提供します。

**主な機能**:
- サービスメッシュの起動・停止
- `/etc/hosts`管理オプション
- サービスへのアクセス方法（HTTP/gRPC）
- 依存関係チェック
- トラブルシューティング

詳細: `.claude/skills/kubectl-localmesh-operations/SKILL.md`

#### `kubectl-localmesh-envoy-protocols` - Envoyプロトコル設定
kubectl-localmeshにおけるEnvoy HTTPプロトコル設定の実装パターンとトラブルシューティングを提供します。

**主な機能**:
- HTTPプロトコルオプション（`http_protocol_options` vs `http2_protocol_options`）の理解
- `protocol: http/http2/grpc`の使い分けガイド
- サービス互換性の判断方法
- プロトコルエラー（502 Bad Gateway、gRPC接続エラー）の診断と解決
- 実装パターンとコード例

詳細: `.claude/skills/kubectl-localmesh-envoy-protocols/SKILL.md`

#### `kubectl-localmesh-logging-guide` - ログ設計ガイド
kubectl-localmeshにおけるログレベル設計とユーザーフレンドリーな出力のガイドラインを提供します。

**主な機能**:
- ログレベル階層（warn/info/debug）の設計
- 起動サマリーの見方
- `internal/log.Logger`の使用パターン
- トラブルシューティング例
- forbidigo lintルールの説明

詳細: `.claude/skills/kubectl-localmesh-logging-guide/SKILL.md`

#### `kubectl-localmesh-gcp-ssh-tunnel` - GCP SSH tunnel実装詳細
GCP SSH Bastion経由のDB接続に関するSSH tunnel実装の詳細、デバッグ方法、既知の問題を提供します。

**主な機能**:
- IAP TCP Tunnel v4プロトコルの仕様
- GCP OS LoginのSSHユーザー名解決の仕組み
- `gcloud compute ssh`とGo SDK実装の違い
- デバッグ時のチェックポイント
- 既知の問題と解決状況

詳細: `.claude/skills/kubectl-localmesh-gcp-ssh-tunnel/SKILL.md`

#### `kubectl-localmesh-macos-localhost` - macOS .localhostドメインの挙動
macOSにおける.localhostドメインの特殊な挙動と、TCPサービス設定時の注意点を提供します。

**主な機能**:
- macOSが`.localhost`を特別扱いする理由（RFC 6761）
- TCPサービスで`.localhost`が動作しない原因と解決策
- 推奨TLD（`.localdomain`など）
- 診断コマンド

詳細: `.claude/skills/kubectl-localmesh-macos-localhost/SKILL.md`

### クイックスタート

```bash
# 1. 依存関係チェック
.claude/skills/kubectl-localmesh-operations/scripts/check-dependencies.sh

# 2. ビルド
task build

# 3. 起動
sudo kubectl-localmesh up -f services.yaml
# または
sudo ./bin/kubectl-localmesh up -f services.yaml
# 位置引数も使用可能
sudo kubectl-localmesh up services.yaml
```

## 設定ファイル形式

**v0.2.0から設定ファイル形式が変更されました。** `kind`フィールドによるタグ付きユニオン型を採用しています。

```yaml
listener_port: 80

# GCP SSH Bastion定義（オプション）
ssh_bastions:
  primary:
    instance: bastion-instance-1    # GCP Compute Instance名
    zone: asia-northeast1-a         # ゾーン
    project: my-gcp-project         # プロジェクトID（省略時は環境変数でフォールバック）
    # ssh_key_path: ~/.ssh/custom-key  # SSH秘密鍵パス（--experimental-ssh用）
    # ssh_user: my-user                # SSHユーザー名（--experimental-ssh用）

services:
  # Kubernetesサービス（HTTP/gRPC）
  - kind: kubernetes                 # 明示的な型区別（kubernetes固定）
    host: users-api.localhost        # ローカルアクセス用ホスト名
    namespace: users                 # K8s namespace
    service: users-api               # K8s Service名
    port_name: grpc                  # Serviceのport名（複数ポートがある場合）
    protocol: grpc                   # http|grpc

  - kind: kubernetes
    host: admin.localhost
    namespace: admin
    service: admin-web
    port: 8080                       # 明示的なポート番号指定も可能
    protocol: http

  # DB接続（TCP via GCP SSH Bastion）
  - kind: tcp                        # 明示的な型区別（tcp固定）
    host: users-db.localhost
    ssh_bastion: primary             # ssh_bastions mapのkey
    target_host: 10.0.0.1            # Private IP (Cloud SQL等)
    target_port: 5432                # DBポート
```

### JSON Schema

設定ファイルのJSON Schemaが`schemas/config.schema.json`に定義されている。

**用途:**
- エディタ統合: `yaml-language-server`による自動補完・検証
- CLI検証: `validate --strict`コマンドによる厳密検証
- 未知フィールド検出: `additionalProperties: false`でタイポを検出

**設定ファイルにスキーマを関連付ける:**
```yaml
# yaml-language-server: $schema=schemas/config.schema.json
listener_port: 80
services: ...
```

**設定ファイルフォーマット変更時の注意:**
設定ファイルの構造に変更が生じる場合は、以下を必ず点検すること:
1. `schemas/config.schema.json`の更新が必要かどうか
2. スナップショットテストの更新が必要かどうか（`-update`フラグで再生成）

### 構造体階層

**v0.2.0でタグ付きユニオン型を導入:**

```
Service (interface)
  └─ GetHost() string
  └─ GetKind() string
  └─ Validate(*Config) error

ServiceDefinition (tagged union root)
  └─ service Service  // 内部フィールド
  └─ Get() Service
  └─ AsKubernetes() (*KubernetesService, bool)
  └─ AsTCP() (*TCPService, bool)
  └─ UnmarshalYAML()  // kind判別
  └─ MarshalYAML()    // kind自動付与

KubernetesService (kubectl port-forward)
  └─ Host, Namespace, Service, PortName, Port, Protocol

TCPService (GCP SSH tunnel)
  └─ Host, SSHBastion, TargetHost, TargetPort
```

**利点:**
- **型安全性**: コンパイル時に型制約を検証
- **明確なバリデーション**: 各サービス型が自身のValidate()を持つ
- **拡張性**: 新しいkind追加が容易
- **可読性**: type switchで処理が明確

## 依存関係

- **Runtime:**
  - `kubectl`: Kubernetesクラスタへのアクセス
  - `envoy`: ローカルプロキシとして動作（macOS: `brew install envoy`）
  - `bash`: port-forwardループスクリプト実行
  - **Kubernetes 1.30+**: WebSocket port-forward対応が必須
  - **GCP SSH Bastion (オプション):**
    - `gcloud` CLI（デフォルトのSSH tunnel実装で使用）
    - Application Default Credentials (ADC)
      - `gcloud auth application-default login` または
      - 環境変数 `GOOGLE_APPLICATION_CREDENTIALS`
    - **`--experimental-ssh`フラグ:** Go SDKによる純Go実装（gcloud CLI不要、experimental）
      - SSH秘密鍵（`~/.ssh/google_compute_engine`、`~/.ssh/id_ed25519`、`~/.ssh/id_rsa`のいずれか）が必要

- **Go modules:**
  - `gopkg.in/yaml.v3`: 設定ファイルパース
  - `k8s.io/client-go v0.35.0+`: Kubernetes client with WebSocket support
  - `github.com/santhosh-tekuri/jsonschema/v6`: JSON Schema Draft 2020-12検証（validateコマンド用）
  - `golang.org/x/crypto/ssh`: SSH client（GCP SSH Bastion experimental用）
  - `golang.org/x/oauth2`: ADC認証（GCP SSH Bastion experimental用）
  - `github.com/gorilla/websocket`: IAP TCP Tunnelプロトコル（GCP SSH Bastion experimental用）

- **開発ツール (aqua管理):**
  - `task`: タスクランナー（Taskfile.yaml実行）
  - `golangci-lint`: Go静的解析ツール
  - `goreleaser`: リリース自動化ツール

開発ツールのインストール:

```bash
aqua install
```

### WebSocket Port-Forward

**重要:** このプロジェクトは、Kubernetes 1.29+でSPDYが非推奨となったため、WebSocketベースのport-forwardを使用しています。

- **最小Kubernetesバージョン**: 1.30+ (WebSocket port-forward対応)
- **実装**: `internal/k8s/portforward.go`で`portforward.NewSPDYOverWebsocketDialer`を使用
- **プロトコル**: WebSocket (RFC 6455) over HTTP/1.1
- **下位互換性**: Kubernetes 1.29以前のクラスタはサポートされません

参考資料:
- [Kubernetes 1.31: WebSockets Transition](https://kubernetes.io/blog/2024/08/20/websockets-transition/)
- [client-go portforward package](https://pkg.go.dev/k8s.io/client-go/tools/portforward)

## 重要な実装詳細

### port-forward自動再接続

`internal/pf/forward.go`では、bashスクリプト経由でwhileループを使用:
```bash
while true; do
  kubectl -n <namespace> port-forward svc/<service> <local>:<remote> || true
  sleep 0.3
done
```

コンテキストキャンセル時に自動終了します。

### Envoy設定の動的生成

- すべてのupstreamクラスタでHTTP/2が有効化されている（`http2_protocol_options`）
- これによりgRPCトラフィック（h2c）をプロキシ可能
- `timeout: 0s`で長時間接続（streaming）をサポート

### クリーンアップ

- `run.Run()`は一時ディレクトリ（`kubectl-localmesh-*`）を作成し、終了時に削除
- すべてのport-forwardプロセスは`ctx`のキャンセルで停止
- Envoyプロセスも`CommandContext`で管理

### /etc/hosts自動管理

`internal/hosts/hosts.go`および`internal/hosts/validate.go`でマーカーコメントを使用した安全な管理:

```
# kubectl-localmesh: managed by kubectl-localmesh
127.0.0.1 users-api.localhost
127.0.0.1 billing-api.localhost
# kubectl-localmesh: end
```

**基本動作:**
- デフォルトでは/etc/hostsを自動的に更新（`--no-edit-hosts`でスキップ可能）
- 通常起動時は自動的に/etc/hostsを更新（sudo必要）
- 終了時（Ctrl+C）に自動クリーンアップ
- `dump-envoy-config`サブコマンドでは更新しない
- 一時ファイル経由で安全に書き換え

**保守的な編集ポリシー（重要）:**
- **無効状態を検出したら自動修復しない** - ユーザーに手動修正を要求
- マーカーブロックが既に存在する場合は起動を拒否（二重起動防止）
- 未完結ブロック、孤立した終了マーカー、ネストしたマーカーを検出
- エラーメッセージで具体的な問題（行番号付き）と修正手順を提示
- `/etc/hosts`の全内容をダンプして問題箇所を明示
- validation型は全てpackage private（内部実装の詳細）

## 今後の拡張

READMEに記載されているロードマップ:
- krew配布
- ✅ サブコマンド（`up`と`dump-envoy-config`実装済み、`down`と`status`は計画中）
- TLS対応（ローカル証明書）
- gRPC-web対応
- Envoy不要のHTTP専用モード
- 設定のホットリロード
