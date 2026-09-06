#!/bin/bash
set -euo pipefail

KUBECONFIG_PATH="/output/kubeconfig.yaml"
KUBECONFIG_TIMEOUT=60  # seconds

echo "Waiting for kubeconfig (timeout: ${KUBECONFIG_TIMEOUT}s)..."
elapsed=0
while [ ! -f "${KUBECONFIG_PATH}" ]; do
    sleep 1
    elapsed=$((elapsed + 1))
    if [ $elapsed -ge $KUBECONFIG_TIMEOUT ]; then
        echo "ERROR: Timeout waiting for kubeconfig after ${KUBECONFIG_TIMEOUT}s"
        exit 1
    fi
done
echo "kubeconfig found"

# kubeconfigのサーバーアドレスをk3sに変更
echo "Updating kubeconfig server address..."
if ! sed -i 's/127.0.0.1/k3s/g' "${KUBECONFIG_PATH}"; then
    echo "ERROR: Failed to update kubeconfig server address"
    exit 1
fi
export KUBECONFIG="${KUBECONFIG_PATH}"

# K8sクラスタが準備できるまで待機
echo "Waiting for K8s API server..."
until kubectl get nodes >/dev/null 2>&1; do
    sleep 2
done
echo "K8s API server ready"

# テストサービスをデプロイ
echo "Deploying test services..."
kubectl apply -k /manifests/

# Deployment の rollout を待つ。`kubectl wait pod` は対象 Pod が 1 つも存在しない間は
# タイムアウトを待たずに "no matching resources found" で即失敗するため、apply 直後の
# Pod がまだ生成されていない時間帯と競合する。
echo "Waiting for pods to be ready..."
if ! kubectl rollout status deployment/http-test-server -n e2e-test --timeout=120s; then
    echo "ERROR: http-test-server pod failed to become ready"
    echo "=== Pod status ==="
    kubectl get pods -n e2e-test -l app=http-test-server -o wide || true
    echo "=== Pod describe ==="
    kubectl describe pod -n e2e-test -l app=http-test-server || true
    echo "=== Pod logs ==="
    kubectl logs -n e2e-test -l app=http-test-server --tail=50 || true
    exit 1
fi

if ! kubectl rollout status deployment/grpc-test-server -n e2e-test --timeout=120s; then
    echo "ERROR: grpc-test-server pod failed to become ready"
    echo "=== Pod status ==="
    kubectl get pods -n e2e-test -l app=grpc-test-server -o wide || true
    echo "=== Pod describe ==="
    kubectl describe pod -n e2e-test -l app=grpc-test-server || true
    echo "=== Pod logs ==="
    kubectl logs -n e2e-test -l app=grpc-test-server --tail=50 || true
    exit 1
fi
echo "Test services ready"

# kubectl-localmesh を起動
echo "Starting kubectl-localmesh..."
exec kubectl-localmesh "$@"
