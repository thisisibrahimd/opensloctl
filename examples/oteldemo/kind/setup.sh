#!/usr/bin/env bash
# Create a kind cluster, render the otel-demo chart via helm, then
# use kustomize to patch the prometheus Deployment + ConfigMap
# (mount openslo-rules ConfigMap; inject rule_files:), then apply
# with kubectl.
#
# Why not `helm install` directly? The chart's values don't expose
# knobs for mounting our rules ConfigMap onto the prometheus pod or
# for adding a top-level `rule_files:` to the chart's prometheus
# ConfigMap. Patches are easier to maintain in kustomize YAML than
# in a custom post-renderer script. Dropping helm install in favor
# of `kubectl apply` sidesteps the post-renderer plumbing entirely
# and keeps the manifest rebuild-testable with a single kustomize
# command.
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
CLUSTER="opensloctl-demo"
NAMESPACE="default"
HELM_RELEASE="otel-demo"
RULES_CM="openslo-rules"
DASHBOARDS_CM="openslo-dashboards"
RULES_MOUNT="/etc/prometheus/openslo-rules"
KIND_DIR="${REPO_ROOT}/examples/oteldemo/kind"
RENDER_DIR="${REPO_ROOT}/tmp"

cd "${REPO_ROOT}"

echo "==> Creating kind cluster '${CLUSTER}'..."
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER}$"; then
  echo "    (already exists, reusing)"
else
  kind create cluster \
    --name "${CLUSTER}" \
    --config "${KIND_DIR}/kind-config.yaml"
fi

kubectl config use-context "kind-${CLUSTER}" >/dev/null

echo "==> Adding OpenTelemetry helm repo + pulling values..."
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts 2>/dev/null || true
helm repo update >/dev/null

mkdir -p "${RENDER_DIR}"
echo "==> Rendering helm chart with values.yaml..."
helm template "${HELM_RELEASE}" open-telemetry/opentelemetry-demo \
  --namespace "${NAMESPACE}" \
  --values "${KIND_DIR}/values.yaml" \
  > "${RENDER_DIR}/helm-rendered.yaml"

echo "==> Patching prometheus Deployment via kustomize (autoloaded rule_files mounted at /etc/prometheus/openslo-rules/)..."
kustomize build --load-restrictor=LoadRestrictionsNone "${KIND_DIR}" \
  > "${RENDER_DIR}/manifest.yaml"

# Inject the openslo-rules path into the chart's prometheus ConfigMap
# rule_files list. The chart ships 4 entries under /etc/config/... for
# helm-managed recording+alerting files; we add ours next to them. A
# duplicate `rule_files:` top-level key fails Prom's strict YAML parser
# so we append directly to the existing block. Idempotent: if our line
# is already present, awk sees it inside the pattern and replaces
# with itself.
#
# Note: if the chart ever changes the rule_files block signature, this
# regex stops matching and the prom container logs a YAML parse error
# on reload. Update patterns to match.
echo "==> Appending openslo-rules to prometheus rule_files (awk)..."
awk -v mount="${RULES_MOUNT}" '
  $0 == "    - /etc/config/alerts" {
    print
    print "    - " mount "/*.yaml"
    next
  }
  { print }
' "${RENDER_DIR}/manifest.yaml" > "${RENDER_DIR}/manifest.new"
mv "${RENDER_DIR}/manifest.new" "${RENDER_DIR}/manifest.yaml"

echo "==> Applying manifest to cluster..."
kubectl apply -f "${RENDER_DIR}/manifest.yaml"

echo "==> Loading OpenSLO Prometheus rules ConfigMap..."
kubectl create configmap "${RULES_CM}" \
  --namespace "${NAMESPACE}" \
  --from-file="${REPO_ROOT}/examples/oteldemo/rules/" \
  --dry-run=client -o yaml \
  | kubectl apply -f -

echo "==> Loading OpenSLO Grafana dashboards ConfigMap (with sidecar label)..."
kubectl create configmap "${DASHBOARDS_CM}" \
  --namespace "${NAMESPACE}" \
  --from-file="${REPO_ROOT}/deploy/dashboards/" \
  --dry-run=client -o yaml \
  | kubectl apply -f -
kubectl label configmap "${DASHBOARDS_CM}" -n "${NAMESPACE}" grafana_dashboard=1 --overwrite

echo "==> Waiting for prometheus to roll out with rules ConfigMap mount..."
kubectl rollout status deployment/prometheus -n "${NAMESPACE}" --timeout=5m

echo "==> Reloading Prometheus via POST /-/reload..."
kubectl exec -n "${NAMESPACE}" deploy/prometheus -- \
  wget -q -O- --post-data='' http://localhost:9090/-/reload >/dev/null \
  || true

echo "==> Port-forwarding Grafana, Prometheus, frontend-proxy to localhost..."
kubectl port-forward -n "${NAMESPACE}" svc/prometheus     9090:9090 \
  > "${RENDER_DIR}/openslo-prometheus-pf.log"     2>&1 &
kubectl port-forward -n "${NAMESPACE}" svc/frontend-proxy 8080:8080 \
  > "${RENDER_DIR}/openslo-frontendproxy-pf.log"  2>&1 &

echo
echo "Demo ready (port-forwards running in background; teardown.sh or kill them)."
echo "  Demo UI       http://localhost:8080/"
echo "    /grafana    http://localhost:8080/grafana/"
echo "    /jaeger/ui  http://localhost:8080/jaeger/ui"
echo "    /feature    http://localhost:8080/feature"
echo "  Direct Graf   http://localhost:3000/"
echo "  Direct Prom   http://localhost:9090/rules"
echo
echo "After re-running 'make generate' to update rules: examples/oteldemo/kind/sync.sh"
echo "Cluster: ${CLUSTER}  Context: kind-${CLUSTER}"
