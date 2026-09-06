# opensloctl

Generate Prometheus recording rules and alerting rules from OpenSlo specs.

## Table of Contents

- [Installation](#installation)
- [Development](#development)
- [Commands](#commands)
- [Usage](#usage)
  - [Recording Rules](#recording-rules)
  - [Alert Rules](#alert-rules)
- [Alerting](#alerting)
  - [Supported strategies](#supported-strategies-conditionkind)
  - [error-rate](#error-rate--sre-%C2%A7%C2%A7-13)
  - [burn-rate](#burn-rate--sre-%C2%A74)
  - [multi-burn-rate](#multi-burn-rate--sre-%C2%A75)
  - [multi-window-multi-burn-rate](#multi-window-multi-burn-rate--sre-%C2%A76)
  - [Alert naming](#alert-naming)
  - [Field reference](#field-reference)
  - [Validation](#validation-1)
  - [Run the examples](#run-the-examples)
- [Multi-dimensional SLIs](#multi-dimensional-slis)
- [Semantic Conventions](#semantic-conventions)

## Installation

### Via mise (GitHub backend)

If you use [mise](https://mise.jdx.dev/), you can install opensloctl directly from GitHub releases:

```
mise use github:thisisibrahimd/opensloctl
```

This adds the tool to your local `mise.toml` and installs the latest release binary. After that, `opensloctl` is available on your PATH within the project.

### From source

```
go install github.com/thisisibrahimd/opensloctl@latest
```

Or clone and build:

```
git clone https://github.com/thisisibrahimd/opensloctl.git
cd opensloctl
go build -o opensloctl .
```

## Development

This project uses [mise](https://mise.jdx.dev/) to manage tool versions (Go, golangci-lint, Weaver).

### Install mise

See [mise installation docs](https://mise.jdx.dev/getting-started.html).

### Install project tools

Once mise is installed, run this in the repo root:

```
mise install
```

This installs the exact versions declared in `mise.toml`:
- **Go** 1.26
- **golangci-lint** (latest)
- **Weaver** (latest) — for semantic convention registry management

After installing, commands like `go`, `golangci-lint`, and `weaver` are available automatically in the project directory.

## Commands

```
go build -o opensloctl .          # build binary
go run . load -f <file>           # parse and print OpenSlo specs
go run . validate -f <file>       # validate specs without writing files
go run . generate -f <file> -o <dir>  # generate Prometheus recording rules
make semconv-generate             # regenerate semconv_gen.go from registry
make semconv-check                # validate registry schema
make lint                         # run golangci-lint
make test                         # run go test ./...
```

## Usage

opensloctl reads OpenSlo SLO, SLI, AlertCondition, and AlertPolicy specs and renders each SLO into a single unified Prometheus rules file named `<slo-name>-rules.yaml`.

### Recording Rules

For each SLO, opensloctl generates Prometheus recording rules that:

1. **Expose SLO metadata** — `openslo_slo_info`, `openslo_slo_objective`, `openslo_slo_timewindow_days`, `openslo_slo_error_budget`
2. **Pre-compute SLI error rates** — `openslo_sli_error_rate_5m`, `_30m`, `_1h`, `_2h`, `_6h`, `_1d`, `_3d`, `_7d`, `_28d`, `_30d`
3. **Event rate** (RatioMetric SLIs) — `openslo_sli_event_rate_<window>` per multi-window: events-per-second from the underlying SLI source query. Lets Grafana panels show traffic context for budget-burn interpretation.
4. **Categorical status** — `openslo_slo_status` ∈ `{0, 1, 2, 3}` derived from the current burn rate against overridable thresholds (see [Status gauge](#status-gauge) below).

The SLI error rate metrics are computed from your Prometheus query with window variables templated in. For example, if your SLI query is:

```promql
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{job="api"}[{{.Window}}])) by (le))
```

The generator produces a recording rule for each window:

```yaml
groups:
  - name: openslo-sli-recordings-api-latency-slo
    rules:
    - record: openslo_sli_error_rate_5m
      expr: histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{job="api"}[5m])) by (le))
      labels:
        openslo_slo_name: api-latency-slo
    - record: openslo_sli_error_rate_30m
      expr: histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{job="api"}[30m])) by (le))
      labels:
        openslo_slo_name: api-latency-slo
    - record: openslo_sli_event_rate_5m           # RatioMetric only
      expr: sum(rate(http_requests_total[5m]))
      labels:
        openslo_slo_name: api-latency-slo
```

Multiline queries are preserved using YAML block scalars (`|`):

```yaml
    - record: openslo_sli_error_rate_5m
      expr: |
        histogram_quantile(0.99,
          sum(rate(http_request_duration_seconds_bucket{job="api"}[5m])) by (le))
```

#### Status gauge

`openslo_slo_status` is a single integer gauge per SLO carrying the categorical health state. Dashboard-friendly because one lookup per SLO replaces three threshold comparisons:

| Value | Label   | Trigger |
|-------|---------|---------|
| `0`   | Healthy | `openslo_slo_current_burn_rate < warning` |
| `1`   | Burning | `warning ≤ current_burn_rate < critical` |
| `2`   | Critical | `critical ≤ current_burn_rate < breached` |
| `3`   | Breached | `current_burn_rate ≥ breached` |

Defaults follow Google SRE Workbook burn-rate reference points: **warning = 1×, critical = 6×, breached = 14.4×**. Override any threshold via SLO annotations:

```yaml
metadata:
  annotations:
    threshold.status.openslo.com/warning: "1"
    threshold.status.openslo.com/critical: "6"
    threshold.status.openslo.com/breached: "14.4"
```

Each annotation is optional; missing ones fall back to defaults independently. The resolved triple must be strictly ascending and positive; otherwise `opensloctl validate` exits 1. Non-numeric values fall back to defaults silently so scratch notes (`TODO`) don't fail validation.

### Alert Rules

When an SLO references AlertPolicies, opensloctl adds an `openslo-alerts-<slo-name>` group to the same `<slo-name>-rules.yaml` file. The unified rules file contains:

1. The SLO info recordings (`openslo_slo_info`, `openslo_slo_objective`, `openslo_slo_error_budget`, …)
2. The windowed SLI error rate recordings (`openslo_sli_error_rate_5m`, `_1h`, …)
3. When alerts are configured, an `openslo-alerts-<slo-name>` group with one Prometheus alert per severity

Everything comes from one Go template (`templates/prometheus-recording-rules.template.yaml`). SLOs without alert policies just emit the recording groups; only filename `openslo-alerts-<slo-name>` group section is omitted.

opensloctl groups the conditions by `severity` and emits one Prometheus alert per severity. The condition's `kind` selects the alerting strategy (see [Alerting](#alerting) for the four supported kinds). Severity stays in the `severity` label; the alert name is `{SloNamePascal}{KindPascal}` and is shared across severities for a given SLO+kind (Prometheus deduplicates same-name alerts within one group, so this is intentional).

## Alerting

opensloctl generates Prometheus alerting rules from OpenSlo `AlertCondition` and `AlertPolicy` specs. The condition's `kind` selects one of four strategies inspired by the [Google SRE Workbook alerting on SLOs](https://sre.google/workbook/alerting-on-slos/) chapters — pick the one that matches how aggressively you want to be paged.

Indicators can be inlined on the SLO (`spec.indicator: ...`) or referenced via `spec.indicatorRef` — referenced SLIs are resolved at generation time and treated as if they were inlined. Inline indicator takes precedence when both are set.

### Supported strategies (`condition.kind`)

| Kind | SRE workbook | Used for |
|---|---|---|
| `error-rate` | §§ 1–3 | Raw SLI error rate vs an absolute threshold (e.g. `0.001` for a 99.9 % SLO). One alert per severity; simplest possible setup. |
| `burn-rate` | § 4 | Single-window burn rate multiplier over the error budget. One alert per severity. |
| `multi-burn-rate` | § 5 | Two or more burn rate windows OR-ed together. Each condition contributes one expression; no short/long pairing. |
| `multi-window-multi-burn-rate` | § 6 | Short+long window pairs AND-ed within a tier, OR-ed across tiers. The recommended pattern when you can afford two windows per tier. |

> The OpenSLO SDK only models the legacy `burnrate` kind. opensloctl accepts the four kebab-case names above and the legacy `burnrate` for back-compat — `burnrate` is mapped to `multi-window-multi-burn-rate` with a one-shot warning at generation time.

### How It Works

1. Define **AlertConditions** with one of the four kinds below.
2. Wrap each condition in an **AlertPolicy** (OpenSLO allows one condition per policy — repeated policies orchestrate the OR/AND logic).
3. Reference policies from your **SLO** via `spec.alertPolicies[]`.
4. opensloctl groups policies by severity, applies the kind's tiering logic, and emits one Prometheus alert per severity.

### `error-rate` — SRE §§ 1–3

Compares the SLI error rate directly to an absolute threshold. The cheapest alert to write — one condition per severity, no tiering. This is the closest mapping to the workbook's "target error rate" / "increased alert window" / "alert on incrementing duration" patterns; the three differ only in `lookbackWindow` length and the presence of `alertAfter` (which maps to Prom `for:`).

```yaml
apiVersion: openslo/v1
kind: AlertCondition
metadata:
  name: api-latency-page
spec:
  severity: page
  condition:
    kind: error-rate
    op: gte
    threshold: 0.001         # absolute error rate (1 - 0.999 for a 99.9 % SLO)
    lookbackWindow: 5m
    alertAfter: 2m           # optional → Prom `for:` clause
```

Generates:

```yaml
- alert: ApiLatencySloErrorRate
  expr: openslo_sli_error_rate_5m{openslo_slo_name="api-latency-slo"} >= 0.001000
  for: 2m
  labels:
    severity: page
    openslo_slo_name: api-latency-slo
```

### `burn-rate` — SRE § 4

Single-window burn rate multiplier over the error budget. One condition per severity; the simplest meaningful burn alert.

```yaml
apiVersion: openslo/v1
kind: AlertCondition
metadata:
  name: checkout-page
spec:
  severity: page
  condition:
    kind: burn-rate
    op: gte
    threshold: 14.4
    lookbackWindow: 5m
    alertAfter: 2m
```

Generates:

```yaml
- alert: CheckoutPageBurnRate
  expr: openslo_sli_error_rate_5m{openslo_slo_name="checkout"} / (1 - openslo_slo_objective{openslo_slo_name="checkout"}) >= 14.400000
  for: 2m
  labels:
    severity: page
    openslo_slo_name: checkout
```

### `multi-burn-rate` — SRE § 5

Two or more burn rate windows OR-ed per severity. No short/long AND pairing — each condition contributes one expression and the alert fires when any condition fires.

```yaml
# alert-condition-page-36x.yaml
apiVersion: openslo/v1
kind: AlertCondition
metadata:
  name: api-latency-page-36x
spec:
  severity: page
  condition:
    kind: multi-burn-rate
    op: gte
    threshold: 36
    lookbackWindow: 5m
---
# alert-condition-page-6x.yaml
apiVersion: openslo/v1
kind: AlertCondition
metadata:
  name: api-latency-page-6x
spec:
  severity: page
  condition:
    kind: multi-burn-rate
    op: gte
    threshold: 6
    lookbackWindow: 30m
```

Generates (conditions OR-ed per severity):

```yaml
- alert: ApiLatencySloMultiBurnRate
  expr: |-
    (openslo_sli_error_rate_5m{openslo_slo_name="api-latency-slo"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency-slo"}) >= 36.000000)
    or
    (openslo_sli_error_rate_30m{openslo_slo_name="api-latency-slo"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency-slo"}) >= 6.000000)
  for: ""
  labels:
    severity: page
    openslo_slo_name: api-latency-slo
```

Structural rule: each severity needs ≥ 2 conditions. If you only want one threshold, use `burn-rate` instead.

### `multi-window-multi-burn-rate` — SRE § 6 (recommended)

Short + long window pairs AND-ed within a tier, OR-ed across tiers. This is what the workbook recommends for production SLOs. The classic fast/slow tiered setup:

```yaml
# Tier 1: fast burn — catches a sudden spike
- name: page-fast-5m      # tier "page-fast" (strip "-5m" suffix)
  severity: page
  condition:
    kind: multi-window-multi-burn-rate
    threshold: 14.4
    lookbackWindow: 5m
    alertAfter: 2m
- name: page-fast-1h      # tier "page-fast" (strip "-1h" suffix)
  severity: page
  condition:
    kind: multi-window-multi-burn-rate
    threshold: 14.4        # same threshold as 5m
    lookbackWindow: 1h
    alertAfter: 2m

# Tier 2: slow burn — catches sustained degradation
- name: page-slow-30m     # tier "page-slow"
  severity: page
  condition:
    kind: multi-window-multi-burn-rate
    threshold: 6
    lookbackWindow: 30m
    alertAfter: 5m
- name: page-slow-6h      # tier "page-slow"
  severity: page
  condition:
    kind: multi-window-multi-burn-rate
    threshold: 6
    lookbackWindow: 6h
    alertAfter: 5m
```

Generates (AND in tier, OR across tiers):

```yaml
- alert: ApiLatencySloMultiWindowMultiBurnRate
  expr: |-
    (
    openslo_sli_error_rate_5m{openslo_slo_name="api-latency-slo"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency-slo"}) >= 14.400000
    and
    openslo_sli_error_rate_1h{openslo_slo_name="api-latency-slo"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency-slo"}) >= 14.400000)
    or
    (
    openslo_sli_error_rate_30m{openslo_slo_name="api-latency-slo"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency-slo"}) >= 6.000000
    and
    openslo_sli_error_rate_6h{openslo_slo_name="api-latency-slo"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency-slo"}) >= 6.000000)
  for: 2m
  labels:
    severity: page
    openslo_slo_name: api-latency-slo
```

Structural rules:

- ≥ 2 tiers per severity (fast + slow, or any other split you want)
- ≥ 2 conditions per tier (a short + a long window) sharing the same threshold
- Condition names must end in `-<lookbackWindow>` for tier derivation (e.g. `page-fast-5m` derives tier `page-fast`). Mismatches are rejected at generate time.

The burn rate values are derived from the error budget math. For a 99.9 % SLO (0.1 % error budget):

| Burn | Budget exhaustion |
|---|---|
| **14.4x** | ~2 hours |
| **6x** | ~5 hours |
| **3x** | ~10 hours |
| **1x** | ~30 days (full period) |

### Choosing a strategy

| Need | Use |
|---|---|
| Simplest possible setup, no burn math | `error-rate` |
| One alert per severity, no AND tie-ups | `burn-rate` |
| Multiple burn windows without short/long pairs | `multi-burn-rate` |
| Robust pattern that catches both spike + sustained burn (recommended) | `multi-window-multi-burn-rate` |

The `examples/oteldemo/specs/` directory uses `multi-window-multi-burn-rate` across 12 SLOs — copy that as a starting point.

### Alert naming

Generated alert names are PascalCase with no separators: `{SloNamePascal}{KindPascal}`.

| Kind | Example alert name |
|---|---|
| `error-rate` | `AdAvailabilityErrorRate` |
| `burn-rate` | `CheckoutPageBurnRate` |
| `multi-burn-rate` | `ApiLatencySloMultiBurnRate` |
| `multi-window-multi-burn-rate` | `AdAvailabilityMultiWindowMultiBurnRate` |

All alerts for one SLO go to a single record group `openslo-alerts-<slo-name>`. Severity is carried in the `severity` label, not in the name — different severities can therefore coexist in the same group under the same alert name (per Prometheus, alert names within a group must be unique; Prometheus's typical recommendation is to differentiate by `severity` label).

### Field reference

| Field | Required | Where | Notes |
|---|---|---|---|
| `kind` | yes | `spec.condition.kind` | One of `error-rate`, `burn-rate`, `multi-burn-rate`, `multi-window-multi-burn-rate` (legacy `burnrate` accepted) |
| `op` | yes | `spec.condition.op` | `gte`, `gt`, `lte`, `lt`. Defaults to `gte`. |
| `threshold` | yes | `spec.condition.threshold` | Absolute (error-rate, must be in `(0, 1]`) or burn multiplier (burn-rate families, must be `> 0`) |
| `lookbackWindow` | yes | `spec.condition.lookbackWindow` | Window duration (e.g. `5m`, `1h`, `6h`) |
| `alertAfter` | no | `spec.condition.alertAfter` | Maps to Prometheus `for:` |
| `severity` | yes | `spec.severity` | `page`, `ticket`, or any custom string (carried in the alert label) |

### Creating AlertPolicies

The OpenSLO SDK only allows one condition per `AlertPolicy`. Repeat policies when you need additional tiers/windows; the generator OR-s across them.

```yaml
apiVersion: openslo/v1
kind: AlertPolicy
metadata:
  name: api-latency-page-fast-5m-policy
spec:
  description: Page tier — fast (5m window)
  alertWhenBreaching: true
  conditions:
    - conditionRef: api-latency-page-fast-5m
  notificationTargets:
    - targetRef: oncall-pagerduty
```

Reference targets with `targetRef:`

```yaml
apiVersion: openslo/v1
kind: AlertNotificationTarget
metadata:
  name: oncall-pagerduty
spec:
  description: Page on-call engineer via PagerDuty
  target: pagerduty
```

### Linking to SLOs

```yaml
apiVersion: openslo/v1
kind: SLO
metadata:
  name: api-latency-slo
spec:
  service: api-gateway
  indicator: ...
  budgetingMethod: Occurrences
  timeWindow:
    - duration: 30d
      isRolling: true
  objectives:
    - displayName: "P99 latency < 500ms"
      target: 0.999
  alertPolicies:
    - alertPolicyRef: api-latency-page-fast-5m-policy
    - alertPolicyRef: api-latency-page-fast-1h-policy
    - alertPolicyRef: api-latency-page-slow-30m-policy
    - alertPolicyRef: api-latency-page-slow-6h-policy
```

### Validation

opensloctl fails fast on load and on generate. Validation rejects:

- Unknown `condition.kind` values
- `error-rate` thresholds outside `(0, 1]`
- `burn-rate`, `multi-burn-rate`, `multi-window-multi-burn-rate` thresholds ≤ 0
- `multi-burn-rate` with fewer than 2 conditions per severity
- `multi-window-multi-burn-rate` with fewer than 2 tiers per severity
- `multi-window-multi-burn-rate` tier with only 1 condition (need a short/long pair)
- `multi-window-multi-burn-rate` tier with non-matching thresholds across conditions
- `multi-window-multi-burn-rate` condition name that doesn't end in `-<lookbackWindow>`
- Unresolved refs anywhere in the SLO → AlertPolicy → AlertCondition → AlertNotificationTarget graph
- Label names containing hyphens (Prometheus requires `[a-zA-Z_][a-zA-Z0-9_]*` — use underscores)
- Multi-value labels (more than one entry per label key)

Refs that can't be resolved look like:

```
unresolved references: [unresolved ref: SLO "api-latency-slo" references Service "missing-svc" not found]
```

### Run the examples

Five examples ship with the repo, each showcasing a different kind:

| Directory | Strategy |
|---|---|
| `examples/api-latency-slo/` | `burn-rate` (single threshold per severity) |
| `examples/error-budget-slo/` | `burn-rate` (single threshold per severity) |
| `examples/error-rate-slo/` | `error-rate` (absolute error rate threshold) |
| `examples/multi-burn-slo/` | `multi-burn-rate` (multiple windows OR-ed) |
| `examples/multi-dim-slo/` | `burn-rate` + multi-dim annotation (one SLO → many series) |
| `examples/oteldemo/specs/` | `multi-window-multi-burn-rate` (full tiered setup across 12 SLOs) |

```bash
# Validate specs without generating files (CI-friendly)
opensloctl validate -r -f examples/api-latency-slo
opensloctl validate -r -f examples/error-rate-slo
opensloctl validate -r -f examples/multi-burn-slo
opensloctl validate -r -f examples/oteldemo/specs

# Generate recording rules + alert rules
rm -rf output/ && mkdir output/
opensloctl generate -r -f examples/api-latency-slo -o output/
opensloctl generate -r -f examples/error-rate-slo -o output/
opensloctl generate -r -f examples/multi-burn-slo -o output/
opensloctl generate -r -f examples/oteldemo/specs -o output/
ls output/
```

Each SLO produces a single `<slo-name>-rules.yaml` containing its recording rules and (if alert policies are referenced) alert rules in one file.

## Multi-dimensional SLIs

A single SLO can be expanded into many recording rule series — one per value of a chosen Prometheus label — by setting two annotations on the SLO's `metadata.annotations`. This is useful when one underlying metric naturally produces many series (per-customer, per-region, per-route, per-caller service) and you want shared SLO target definitions across all of them with per-dimension burn visibility.

### Annotations

| Annotation | Required | Role |
|---|---|---|
| `multi-dimensional-sli.openslo.com/label` | yes | The Prometheus label whose value is joined into `openslo_slo_name` to produce one series per value |
| `multi-dimensional-sli.openslo.com/dimensions` | info only | Human-readable list of dimension values; not used by the generator |

### How it works

When both annotations are set, the generator emits two layers of recording rules for the SLO:

1. **Base `_unlabeled` recordings** — each metric (`openslo_slo_info`, `openslo_slo_objective`, `openslo_slo_timewindow_days`, `openslo_slo_error_budget`, `openslo_slo_current_burn_rate`, `openslo_slo_period_burn_rate`, `openslo_slo_period_error_budget_remaining`, and every `openslo_sli_error_rate_*`) is emitted with the `_unlabeled` suffix. They carry only `openslo_slo_name` and `openslo_spec_version` labels; the chosen dimension label flows through from the underlying source query's series.
2. **Post-process `label_join` rules** — sibling rules that join the value of the chosen dimension label into `openslo_slo_name` with `-` as the separator, producing one series per dimension value.

After Prometheus evaluates the rules, you see one series per dimension value, e.g. for `service_name=account,checkout,recommendation`:

```
openslo_slo_current_burn_rate{openslo_slo_name="account-api-latency"}      = 0.2
openslo_slo_current_burn_rate{openslo_slo_name="checkout-api-latency"}     = 1.4
openslo_slo_current_burn_rate{openslo_slo_name="recommendation-api-latency"} = 4.7
```

### Example spec

```yaml
apiVersion: openslo/v1
kind: SLO
metadata:
  name: api-latency
  annotations:
    multi-dimensional-sli.openslo.com/label: service_name
    multi-dimensional-sli.openslo.com/dimensions: "account,checkout,recommendation"
spec:
  service: api-gateway
  indicator:
    metadata:
      name: api-gateway-latency-sli
    spec:
      thresholdMetric:
        metricSource:
          type: Prometheus
          spec:
            query: histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{job="api-gateway"}[{{.Window}}])) by (le))
  objectives:
    - displayName: "P99 latency under 500ms"
      target: 0.999
```

A complete working example lives at [`examples/multi-dim-slo/`](examples/multi-dim-slo/).

### When to use

Use multi-dimensional SLIs when:

- The underlying metric already splits by a label and you want shared target definitions across all series.
- Alert routing benefits from per-dimension firing rather than aggregate (e.g. per-caller-service paging).
- You want per-dimension burn dashboards without writing one SLO per dimension.

Skip when:

- The cardinal label is unbounded (`user_id`, raw trace IDs) — recording-rule count grows linearly with cardinality and burns Prometheus.
- You only care about the aggregate across all series — a regular single-dim SLO is simpler.

### Trade-offs

- Recording rule count grows linearly with the cardinality of the chosen label. Pick a bounded label with a known max (caller services, regions, routes).
- The dimension value is embedded in `openslo_slo_name` rather than as a separate label — dashboard queries must filter by prefix match or use the original source label.
- Alert rule names (`ApiLatencySloBurnRate`) span all dimension values; severity in the `severity` label differentiates per-dimension alerts.

## Semantic Conventions

opensloctl defines a registry of metrics and attributes for SLO telemetry. The registry lives in `semconv/registry/` and is used to generate `pkg/semconv/semconv_gen.go`.

### Attributes

| Attribute | Type | Description |
|---|---|---|
| `openslo.slo.name` | string | The name of the SLO as defined in the OpenSlo spec. |
| `openslo.spec.version` | string | The OpenSlo API version of the SLO spec. |
| `openslo.service.name` | string | The name of the service the SLO belongs to. |
| `openslo.alert.severity` | string | The alert severity (e.g., page, ticket) attached to SLO alert rules. |
| `openslo.notification.target` | string | The notification target for an alert (e.g., pagerduty, slack, engineers). |

Deprecated attributes (still emitted as Go constants for back-compat, marked `deprecated.reason: obsoleted` in the registry):

| Attribute | Type | Reason |
|---|---|---|
| `openslo.objective.decimal` | double | Unused; openslo_slo_objective recording rule carries the value. |
| `openslo.objective.percent` | double | Unused; openslo_slo_objective recording rule carries the value. |
| `openslo.timewindow.duration` | string | Unused; encoded in the SLI error-rate windowed recording rules. |

### Metrics

All metrics carry `openslo.slo.name` and `openslo.spec.version` attributes unless otherwise noted.

#### SLO Info

| Metric | Type | Unit | Description |
|---|---|---|---|
| `openslo.slo.info` | gauge | 1 | Identifies the existence of an SLO. Always has value 1. |
| `openslo.slo.objective` | gauge | 1 | The target SLI objective (e.g., 0.999 for 99.9% availability). |
| `openslo.slo.timewindow_days` | gauge | 1 | The SLO time window duration expressed as a number of days. |
| `openslo.slo.error_budget` | gauge | 1 | The error budget calculated as 1 minus the objective. |
| `openslo.slo.current_burn_rate` | gauge | 1 | Instantaneous error-budget burn rate = 5-minute SLI error rate divided by the error budget. |
| `openslo.slo.period_burn_rate` | gauge | 1 | Period error-budget burn rate = full-window SLI error rate (typically 30d) divided by the error budget. Emitted when an SLO time window matches the multi-window set. |
| `openslo.slo.period_error_budget_remaining` | gauge | 1 | Remaining error budget ratio over the full period = `clamp_min(1 - period_burn_rate, 0)`. Bounded to `[0, 1]` so dashboards don't chart large negatives. |
| `openslo.slo.status` | gauge | 1 | Categorical health state. `0` = Healthy, `1` = Burning, `2` = Critical, `3` = Breached. Driven by `current_burn_rate` against overridable thresholds (defaults `1/6/14.4×` per the SRE workbook). See [Status gauge](#status-gauge) above. |

#### SLI Error Rate

| Metric | Description |
|---|---|
| `openslo.sli.error_rate_5m` | SLI error rate over a 5-minute window. |
| `openslo.sli.error_rate_30m` | SLI error rate over a 30-minute window. |
| `openslo.sli.error_rate_1h` | SLI error rate over a 1-hour window. |
| `openslo.sli.error_rate_2h` | SLI error rate over a 2-hour window. |
| `openslo.sli.error_rate_6h` | SLI error rate over a 6-hour window. |
| `openslo.sli.error_rate_1d` | SLI error rate over a 1-day window. |
| `openslo.sli.error_rate_3d` | SLI error rate over a 3-day window. |
| `openslo.sli.error_rate_7d` | SLI error rate over a 7-day window. |
| `openslo.sli.error_rate_28d` | SLI error rate over a 28-day window. |
| `openslo.sli.error_rate_30d` | SLI error rate over a 30-day window. |

#### SLI Event Rate (RatioMetric SLIs only)

| Metric | Description |
|---|---|
| `openslo.sli.event_rate_5m` … `_30d` | SLI event rate (events/sec) over each multi-window. Emitted only for `RatioMetric` SLIs since the spec exposes a `total` count query. Use these in dashboards to interpret error-budget burn in absolute event-volume terms, not just ratio. ThresholdMetric (histogram-style) SLIs do not emit this metric because the spec lacks a parallel `event_count` query. |

### Registry Management

The semantic convention registry is managed with [OpenTelemetry Weaver](https://github.com/open-telemetry/weaver).

- **Registry source**: `semconv/registry/` — YAML definitions for attributes and metrics
- **Generated code**: `pkg/semconv/semconv_gen.go` — auto-generated Go constants from the registry
- **Templates**: `semconv/templates/go/` — MiniJinja templates that produce the Go file

```
make semconv-generate   # regenerate semconv_gen.go from registry
make semconv-check      # validate registry schema
make semconv-stats      # show registry statistics
make semconv-diff BASE=<ref>  # detect breaking changes vs a base ref
```

### Consuming the Registry

If your project also uses OpenTelemetry Weaver, you can depend on this registry directly. Add it as a dependency in your `manifest.yaml`:

```yaml
schema_url: https://your-org.com/schemas/your-app/v1.0.0

dependencies:
  - schema_url: https://openslo.com/schemas/v1.0.0
    registry_path: https://github.com/thisisibrahimd/opensloctl.git[semconv/registry]
```

Then reference the attributes in your own metrics and spans:

```yaml
metrics:
  - name: myapp.slo.burn_rate
    instrument: gauge
    unit: "1"
    stability: development
    brief: Current error budget burn rate.
    attributes:
      - ref: openslo.slo.name
        requirement_level: required
      - ref: openslo.spec.version
        requirement_level: required
```

Alternatively, if you don't use Weaver, the Go constants are available at `github.com/thisisibrahimd/opensloctl/pkg/semconv`:

```go
import "github.com/thisisibrahimd/opensloctl/pkg/semconv"

// Use generated constants
meter.Float64ObservableGauge(semcov.METRIC_OPENSLO_SLO_INFO)

// Status gauge — single lookup returns 0/1/2/3 against overridable thresholds
gauge := meter.Float64ObservableGauge(semcov.METRIC_OPENSLO_SLO_STATUS,
    api.WithDescription("SLO categorical health state"))

```
