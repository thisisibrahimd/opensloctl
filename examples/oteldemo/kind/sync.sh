#!/usr/bin/env bash
# Re-apply the rules + dashboards ConfigMaps after `make generate`,
# re-add the sidecar label for dashboards, and reload Prometheus.
#
# The chart's prometheus ConfigMap rule_files block stays stable across
# helm upgrades, so we don't re-render or re-patch it here - the
# rule_files entry pointing at our mount was added during setup.sh and
# persists on the cluster between syncs.
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
NAMESPACE="${NAMESPACE:-default}"
RULES_CM="openslo-rules"
DASHBOARDS_CM="openslo-dashboards"

cd "${REPO_ROOT}"

echo "==> Syncing openslo-rules ConfigMap..."
# Replace (delete+create) instead of apply to avoid the
# kubectl.kubernetes.io/last-applied-configuration annotation accumulating
# past the 256 KiB ConfigMap metadata limit when the ruleset grows
# (e.g. adding one rule per SLO makes each apply entry ~30% larger).
kubectl delete configmap "${RULES_CM}" -n "${NAMESPACE}" --ignore-not-found
kubectl create configmap "${RULES_CM}" \
  --namespace "${NAMESPACE}" \
  --from-file="${REPO_ROOT}/examples/oteldemo/rules/" \
  --from-file="${REPO_ROOT}/deploy/rules/"

echo "==> Syncing openslo-dashboards ConfigMap (with sidecar label)..."
kubectl delete configmap "${DASHBOARDS_CM}" -n "${NAMESPACE}" --ignore-not-found
kubectl create configmap "${DASHBOARDS_CM}" \
  --namespace "${NAMESPACE}" \
  --from-file="${REPO_ROOT}/deploy/dashboards/"
kubectl label configmap "${DASHBOARDS_CM}" -n "${NAMESPACE}" grafana_dashboard=1 --overwrite

echo "==> Reloading Prometheus (POST /-/reload)..."
kubectl exec -n "${NAMESPACE}" deploy/prometheus -- \
  wget -q -O- --post-data='' http://localhost:9090/-/reload >/dev/null \
  || true
