#!/bin/bash
# kubectl-local-mesh 依存関係チェックスクリプト

set -e

echo "==================================="
echo "kubectl-local-mesh 依存関係チェック"
echo "==================================="
echo ""

exit_code=0

# kubectl
echo "【kubectl】"
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl が見つかりません"
    echo "   インストール方法: https://kubernetes.io/docs/tasks/tools/"
    exit_code=1
else
    version=$(kubectl version --client --short 2>/dev/null || kubectl version --client 2>&1 | head -n 1)
    echo "✅ kubectl: $version"
fi
echo ""

# envoy
echo "【envoy】"
if ! command -v envoy &> /dev/null; then
    echo "❌ envoy が見つかりません"
    echo "   macOS: brew install envoy"
    echo "   Linux: https://www.envoyproxy.io/docs/envoy/latest/start/install"
    exit_code=1
else
    version=$(envoy --version 2>&1 | head -n 1)
    echo "✅ envoy: $version"
fi
echo ""

# bash
echo "【bash】"
if ! command -v bash &> /dev/null; then
    echo "❌ bash が見つかりません"
    exit_code=1
else
    version=$(bash --version | head -n 1)
    echo "✅ bash: $version"
fi
echo ""

# kubeconfig確認（オプション）
echo "【Kubernetes接続確認（オプション）】"
if kubectl cluster-info &> /dev/null; then
    context=$(kubectl config current-context 2>/dev/null)
    echo "✅ Kubernetesクラスタに接続可能"
    echo "   現在のコンテキスト: $context"
else
    echo "⚠️  Kubernetesクラスタに接続できません"
    echo "   kubectl-local-meshの起動時にエラーになる可能性があります"
    echo "   （オフラインモードでのEnvoy設定ダンプは可能）"
fi
echo ""

# 結果サマリー
echo "==================================="
if [ $exit_code -eq 0 ]; then
    echo "✅ すべての依存関係が満たされています"
    echo ""
    echo "次のステップ:"
    echo "  sudo ./bin/kubectl-local-mesh -f services.yaml"
else
    echo "❌ 必須の依存関係が不足しています"
    echo "   上記のインストール方法を参照してください"
fi
echo "==================================="

exit $exit_code
