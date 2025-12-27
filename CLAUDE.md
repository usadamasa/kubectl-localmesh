# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

`kubectl-local-mesh`は、`kubectl port-forward`をベースにしたローカル専用の疑似サービスメッシュツールです。複数のKubernetesサービスに対して、単一のローカルエントリポイントからホストベースのルーティング（HTTP/gRPC）でアクセスできます。

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

## 開発コマンド

### ビルド

```bash
go build -o kubectl-local-mesh ./cmd/local-mesh
```

### 実行

```bash
# 通常起動（/etc/hostsを自動更新、sudo必要）
sudo ./kubectl-local-mesh -f services.yaml

# /etc/hosts更新を無効化
./kubectl-local-mesh -f services.yaml --update-hosts=false

# ログレベル指定
sudo ./kubectl-local-mesh -f services.yaml -log debug
```

### テスト

```bash
go test ./...
```

現在、テストファイルは存在しませんが、将来的には以下のテスト戦略を推奨:
- `internal/config`: YAML parsing, validation
- `internal/kube`: kubectl command mocking
- `internal/pf`: port allocation logic
- `internal/envoy`: config generation

### Envoy設定のダンプ

Envoy設定をYAML形式で標準出力にダンプ:

```bash
# 基本的な使用方法（クラスタ接続が必要）
./kubectl-local-mesh --dump-envoy-config -f services.yaml

# ファイルにリダイレクト
./kubectl-local-mesh --dump-envoy-config -f services.yaml > envoy-config.yaml
```

### オフラインモード（モック設定）

Kubernetesクラスタに接続せずにEnvoy設定を確認:

```bash
# モック設定ファイルを作成
cat > mocks.yaml <<EOF
mocks:
  - namespace: users
    service: users-api
    port_name: grpc
    resolved_port: 50051
  - namespace: billing
    service: billing-api
    port_name: http
    resolved_port: 8080
EOF

# モック設定を使ってダンプ
./kubectl-local-mesh --dump-envoy-config -f services.yaml --mock-config mocks.yaml
```

モック設定ファイル形式:
- `namespace`, `service`, `port_name`でサービスを識別
- `resolved_port`はkubectl呼び出しの代わりに使用するポート番号

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

- **Go modules:**
  - `gopkg.in/yaml.v3`: 設定ファイルパース

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

- `run.Run()`は一時ディレクトリ（`kubectl-local-mesh-*`）を作成し、終了時に削除
- すべてのport-forwardプロセスは`ctx`のキャンセルで停止
- Envoyプロセスも`CommandContext`で管理

### /etc/hosts自動管理

`internal/hosts/hosts.go`でマーカーコメントを使用した安全な管理:

```
# kubectl-local-mesh: managed by kubectl-local-mesh
127.0.0.1 users-api.localhost
127.0.0.1 billing-api.localhost
# kubectl-local-mesh: end
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
