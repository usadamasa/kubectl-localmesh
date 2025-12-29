# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

`kubectl-localmesh`は、`kubectl port-forward`をベースにしたローカル専用の疑似サービスメッシュツールです。複数のKubernetesサービスに対して、単一のローカルエントリポイントからホストベースのルーティング（HTTP/gRPC）でアクセスできます。

**重要な設計原則:**
- クラスタ側には何もインストールしない（kubectl primitives only）
- ローカル開発・デバッグ専用（本番環境は対象外）
- 障害モードを明確に保つ
- 起動・破棄が容易

## アーキテクチャ

```
クライアント
  ↓ (http://users-api.localhost:80)
ローカルEnvoy (ホストベースルーティング)
  ↓ (動的に割り当てられたローカルポート)
kubectl port-forward (各サービスごとに自動起動・再接続)
  ↓
Kubernetesサービス
```

### 主要コンポーネント

1. **config** (`internal/config/`)
   - YAMLベースの設定ファイル読み込み
   - `listener_port`: Envoyが待ち受けるローカルポート（デフォルト: 80）
   - `services`: ルーティング対象のサービス一覧（host, namespace, service, port/port_name, type）

2. **kube** (`internal/kube/`)
   - `kubectl`コマンドのラッパー
   - サービスのポート解決（port_name指定またはports[0]をフォールバック）
   - JSONPathを使ったService定義の取得

3. **pf** (`internal/pf/`)
   - `kubectl port-forward`のライフサイクル管理
   - 自動再接続ループ（bashスクリプト経由）
   - 動的なローカルポート割り当て（`FreeLocalPort`）

4. **envoy** (`internal/envoy/`)
   - Envoy設定ファイル（YAML）の動的生成
   - HTTP/2対応（h2c plaintext for gRPC）
   - ホストベースのvirtual_hosts設定
   - 各サービスへのSTATICクラスタ定義

5. **hosts** (`internal/hosts/`)
   - /etc/hosts ファイルの管理
   - マーカーコメントによるエントリの追跡・削除
   - 書き込み権限チェック

6. **run** (`internal/run/`)
   - オーケストレーションロジック
   - 各サービスに対してport-forwardプロセスを起動
   - Envoy設定を生成・適用
   - Envoyプロセスの起動・監視
   - クリーンアップ処理

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
- `--dump-envoy-config`: Envoy設定のダンプ
- `--mock-config`: オフラインモード（クラスタ接続不要）
- `-log debug`: 詳細デバッグログ
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

### クイックスタート

```bash
# 1. 依存関係チェック
.claude/skills/kubectl-localmesh-operations/scripts/check-dependencies.sh

# 2. ビルド
task build

# 3. 起動
sudo kubectl localmesh -f services.yaml
# または
sudo ./bin/kubectl-localmesh -f services.yaml
```

## 設定ファイル形式

```yaml
listener_port: 80
services:
  - host: users-api.localhost       # ローカルアクセス用ホスト名
    namespace: users                 # K8s namespace
    service: users-api               # K8s Service名
    port_name: grpc                  # Serviceのport名（複数ポートがある場合）
    type: grpc                       # メタデータ（http/grpc）

  - host: admin.localhost
    namespace: admin
    service: admin-web
    port: 8080                       # 明示的なポート番号指定も可能
    type: http
```

## 依存関係

- **Runtime:**
  - `kubectl`: Kubernetesクラスタへのアクセス
  - `envoy`: ローカルプロキシとして動作（macOS: `brew install envoy`）
  - `bash`: port-forwardループスクリプト実行
  - **Kubernetes 1.30+**: WebSocket port-forward対応が必須

- **Go modules:**
  - `gopkg.in/yaml.v3`: 設定ファイルパース
  - `k8s.io/client-go v0.35.0+`: Kubernetes client with WebSocket support

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

`internal/hosts/hosts.go`でマーカーコメントを使用した安全な管理:

```
# kubectl-localmesh: managed by kubectl-localmesh
127.0.0.1 users-api.localhost
127.0.0.1 billing-api.localhost
# kubectl-localmesh: end
```

- `--update-hosts`フラグのデフォルトは`true`
- 通常起動時は自動的に/etc/hostsを更新（sudo必要）
- 終了時（Ctrl+C）に自動クリーンアップ
- `--dump-envoy-config`モードでは更新しない
- 一時ファイル経由で安全に書き換え

## 今後の拡張

READMEに記載されているロードマップ:
- krew配布
- サブコマンド（up, down, status）
- TLS対応（ローカル証明書）
- gRPC-web対応
- Envoy不要のHTTP専用モード
- 設定のホットリロード
