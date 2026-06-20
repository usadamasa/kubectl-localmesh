# マルチステージビルド: Goビルド
FROM golang:1.26-alpine AS builder

WORKDIR /src

# 依存関係をキャッシュ
COPY go.mod go.sum ./
RUN go mod download

# ソースコードをコピーしてビルド
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /kubectl-localmesh main.go

# 実行イメージ: envoyproxy/envoy ベース
FROM envoyproxy/envoy:v1.32.0

# 必要なツールをインストール
RUN apt-get update && apt-get install -y \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# kubectl インストール (アーキテクチャ自動検出)
RUN ARCH=$(dpkg --print-architecture) && \
    curl -LO "https://dl.k8s.io/release/v1.31.4/bin/linux/${ARCH}/kubectl" \
    && chmod +x kubectl \
    && mv kubectl /usr/local/bin/

# kubectl-localmesh バイナリをコピー
COPY --from=builder /kubectl-localmesh /usr/local/bin/kubectl-localmesh

# entrypointスクリプトをコピー
COPY test/e2e/docker/localmesh/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# 作業ディレクトリ
WORKDIR /app

# entrypointスクリプトを使用
ENTRYPOINT ["/entrypoint.sh"]
