#!/usr/bin/env bash
# Tear down the demo: kill port-forwards, delete the rendered
# manifest from the cluster, then delete the kind cluster.
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
CLUSTER="opensloctl-demo"
RENDER_DIR="${REPO_ROOT}/tmp"

cd "${REPO_ROOT}"

pkill -f "kubectl port-forward.*openslo" 2>/dev/null || true

echo "==> Deleting manifest..."
kubectl delete -f "${RENDER_DIR}/manifest.yaml" --ignore-not-found 2>/dev/null || true

echo "==> Deleting OpenSLO ConfigMaps..."
kubectl delete configmap openslo-rules    -n default --ignore-not-found 2>/dev/null || true
kubectl delete configmap openslo-dashboards -n default --ignore-not-found 2>/dev/null || true

echo "==> Deleting kind cluster '${CLUSTER}'..."
kind delete cluster --name "${CLUSTER}"
