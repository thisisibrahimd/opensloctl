# opensloctl

Generate Prometheus recording rules and alerting rules from OpenSLO specs.

> **Status: active development.** Output shapes, code paths, config conventions, and example layouts can change between minor versions. Pin a specific release if you're relying on it, and expect breaking changes. We're shipping toward a stable `1.0`; right now everything below the generator interface is honest engineering work, not a finished product.
>
> Releases: <https://github.com/thisisibrahimd/opensloctl/releases> - Changelog: [`CHANGELOG.md`](CHANGELOG.md)

## Contents

- [1. Quick start](#1-quick-start)
- [2. What gets generated](#2-what-gets-generated)
- [3. Writing specs](#3-writing-specs)
- [4. Alerting strategies](#4-alerting-strategies)
  - [`error-rate` - SRE §§ 1-3](#error-rate--sre--s-s-1-3)
  - [`burn-rate` - SRE § 4](#burn-rate--sre--s-4)
  - [`multi-burn-rate` - SRE § 5](#multi-burn-rate--sre--s-5)
  - [`multi-window-multi-burn-rate` - SRE § 6](#multi-window-multi-burn-rate--sre--s-6)
  - [Choosing a strategy](#choosing-a-strategy)
  - [Alert naming](#alert-naming)
  - [Field reference](#field-reference)
- [5. Multi-dimensional SLIs](#5-multi-dimensional-slis)
- [6. Dashboards and integrity rules](#6-dashboards-and-integrity-rules)
- [7. Examples](#7-examples)
- [8. Semantic conventions](#8-semantic-conventions)
- [9. Development](#9-development)

## 1. Quick start

Install the binary, validate one of the shipped examples, generate rules, push them into a Prometheus config.

### Install

```
# via mise
mise use github:thisisibrahimd/opensloctl

# or from source
go install github.com/thisisibrahimd/opensloctl@latest
```

### Validate, generate, ship

```
# parses every spec under examples/api-latency-slo/, fails on bad refs or thresholds
opensloctl validate -f examples/api-latency-slo

# writes one rules file per SLO into output/
rm -rf output/ && mkdir output/
opensloctl generate -f examples/api-latency-slo -o output/
ls output/
# api-latency-rules.yaml

# drop the file into Prometheus (rule_files: [./rules/*.yaml])
opensloctl generate -f examples/api-latency-slo -o /etc/prometheus/rules/
curl -X POST http://localhost:9090/-/reload

# confirm the rules loaded
curl -s http://localhost:9090/api/v1/rules | jq '.data.groups[] | .name | select(startswith("openslo-"))'
```

### Checkpoint

Pick the SLO and look up the categorical state:

```
curl -s 'http://localhost:9090/api/v1/query?query=openslo_slo_status' | jq
```

A non-empty result with values in `{0, 1, 2, 3}` means the recording rules loaded and the SLI query resolved in your Prom. See [§2.2](#22-recording-rules) for the full set of generated metrics, [§2.3](#23-prom-3x-compatibility) for the one Prom 3.x subtlety that affects dashboards.

## 2. What gets generated

One file per SLO, named `<slo-name>-rules.yaml`:

```
groups:
  - name: openslo-sli-recordings-<slo-name>      # always present
    rules:
      - openslo_slo_info, openslo_slo_objective, openslo_slo_timewindow_days,
        openslo_slo_error_budget
      - openslo_sli_error_rate_<window> for each window in the multi-window set
      - openslo_sli_event_rate_<window>          # RatioMetric SLOs only
      - openslo_slo_current_burn_rate, openslo_slo_period_burn_rate,
        openslo_slo_period_error_budget_remaining
  - name: openslo-sli-recordings-<slo-name>-unlabeled + label_join
    rules:                                       # multi-dimensional SLOs only
  - name: openslo-alerts-<slo-name>              # only when alertPolicies are referenced
    rules: <one Prom alert per severity>
```

### 2.1 Recording rules

| Metric | Type | What it carries |
|---|---|---|
| `openslo_slo_info` | gauge (=1) | Identity record for the SLO. Carries `openslo_slo_name`, `openslo_slo_description` (folded one-liner from spec), `openslo_service_name`, `openslo_spec_version`. Demo SLOs add `chaos_flag`. |
| `openslo_slo_objective` | gauge | Spec target (e.g. 0.999 for 99.9%). |
| `openslo_slo_timewindow_days` | gauge | Spec `timeWindow` expressed in days (e.g. 28 for 28d). |
| `openslo_slo_error_budget` | gauge | `1 - objective`. (e.g. 0.001 for 99.9%). |
| `openslo_sli_error_rate_<window>` | gauge | SLI error rate over the named window. Windows in the default set: `5m, 30m, 1h, 2h, 6h, 1d, 3d, 7d, 28d, 30d`. |
| `openslo_sli_event_rate_<window>` | gauge (ratio only) | Events/sec over that window. Emitted only for `RatioMetric` SLOs since the spec exposes a `total` count query. |
| `openslo_slo_current_burn_rate` | gauge | `sli_error_rate_5m / error_budget`. Instantaneous burn rate. |
| `openslo_slo_period_burn_rate` | gauge | `sli_error_rate_<longest_window> / error_budget`. Period burn rate over the spec's `timeWindow`. |
| `openslo_slo_period_error_budget_remaining` | gauge | `clamp_min(1 - period_burn_rate, 0)`. Bounded to `[0, 1]` so dashboards don't chart large negatives. |
| `openslo_slo_status` | gauge | Categorical 0-3 health state. See below. |

### 2.2 Status gauge

`openslo_slo_status` is a single integer gauge per SLO, dashboard-friendly because one lookup replaces three threshold comparisons:

| Value | Label | Trigger on `openslo_slo_current_burn_rate` |
|-------|--------|---|
| `0` | Healthy | `< warning` |
| `1` | Burning | `warning <= x < critical` |
| `2` | Critical | `critical <= x < breached` |
| `3` | Breached | `>= breached` |

Defaults follow Google SRE Workbook burn-rate reference points: **warning = 1x, critical = 6x, breached = 14.4x**. Override per-SLO via annotations:

```yaml
metadata:
  annotations:
    threshold.status.openslo.com/warning: "1"
    threshold.status.openslo.com/critical: "6"
    threshold.status.openslo.com/breached: "14.4"
```

Each annotation is optional; missing ones fall back to defaults independently. The resolved triple must be strictly ascending and positive; otherwise `opensloctl validate` exits 1. Scratch notes (`TODO`) don't fail validation - non-numeric values silently fall back.

### 2.3 Prom 3.x compatibility

Every comparison inside the status block uses the `bool` modifier (`>= bool 14.4`, `< bool 6`, etc.). Without `bool`, Prometheus >= 0.19.0 applies default filter semantics at scalar operators - `(burn_rate >= 14.4)` returns the LHS series unchanged instead of `0`/`1` - and the dashboard's `Healthy/Burning/Critical/Breached` mapping never matches. The `bool` modifier has been stable since Prom 0.19.0 (Oct 2015), so the rules work on every Prom version in practical use. Alert expressions deliberately do NOT use `bool` since there, filter semantics is what you want. Test coverage: `internal/generator/prometheusgenerator.TestStatusRuleUsesBoolModifier` regresses every status block for exactly 3 `>= bool` and 2 `<  bool`, and no `bool` survives into alert blocks.

### 2.4 Alert rules

When an SLO references `AlertPolicies`, opensloctl groups them by `severity` and emits one Prometheus alert per severity inside `openslo-alerts-<slo-name>`. The condition `kind` selects the strategy (see [§4](#4-alerting-strategies)). Severity stays in the `severity` label; the alert name is `{SloNamePascal}{KindPascal}` and is shared across severities for a given SLO+kind (Prometheus deduplicates same-name alerts within one group, so this is intentional).

Alert name length grows with the SLO name. `PostOrderEmailLatencySloMultiWindowMultiBurnRate` is 49 characters - well under the typical 63-byte Prometheus label-value cap, but a reason to keep SLO names short if you also rely on `ALERTS{alertname=...}` lookups.

## 3. Writing specs

OpenSlo specs are declarative YAML. opensloctl reads them and emits Prometheus rules; validation happens at load and at generate.

### 3.1 Minimal SLO with inline SLI

```yaml
apiVersion: openslo/v1
kind: SLO
metadata:
  name: api-latency
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
            query: histogram_quantile(0.999, sum(rate(http_request_duration_seconds_bucket{job="api-gateway"}[{{.Window}}])) by (le))
  budgetingMethod: Occurrences
  timeWindow:
    - duration: 30d
      isRolling: true
  objectives:
    - displayName: "P99 latency under 500ms"
      target: 0.999
```

The `Service` definition can be inline (`spec.service: api-gateway`) or referenced (`spec.serviceRef`). Indicator definitions can be inline (`spec.indicator`) or referenced (`spec.indicatorRef`). Each SLO's spec author owns the 0-1 shape of the SLI - opensloctl passes PromQL through verbatim, so `thresholdMetric` queries must already return 0/1 per cycle (or `ratioMetric` must give a [0,1] fraction in the multi-window). Use `ratioMetric(counter=true)` when you can express good/total via classic bucket + calls series.

### 3.2 Multi-line queries

`{{.Window}}` is templated per recording-window. Multiline queries are preserved using YAML block scalars (`|`):

```yaml
- record: openslo_sli_error_rate_5m
  expr: |
    histogram_quantile(0.999,
      sum(rate(http_request_duration_seconds_bucket{job="api-gateway"}[5m])) by (le))
```

### 3.3 AlertCondition + AlertPolicy + SLO wiring

```yaml
apiVersion: openslo/v1
kind: AlertCondition
metadata:
  name: api-latency-page
spec:
  severity: page
  condition:
    kind: burn-rate
    op: gte
    threshold: 14.4
    lookbackWindow: 5m
    alertAfter: 2m           # optional -> Prom `for:`
---
apiVersion: openslo/v1
kind: AlertPolicy
metadata:
  name: api-latency-page
spec:
  description: Page tier (5m window)
  alertWhenBreaching: true
  conditions:
    - conditionRef: api-latency-page   # exactly one; SDK enforces this
  notificationTargets:
    - targetRef: oncall-pagerduty
```

Then on the SLO:

```yaml
spec:
  ...
  alertPolicies:
    - alertPolicyRef: api-latency-page
    - alertPolicyRef: api-latency-ticket
```

OpenSLO SDK v0.9.2 enforces `SliceLength(1,1)` on `AlertPolicy.Spec.Conditions`. Use multiple AlertPolicies (one per condition) to orchestrate OR/AND across tiers and windows.

### 3.4 Validation

opensloctl fails fast on load and on generate. Validation rejects:

- Unknown `condition.kind` values
- `error-rate` thresholds outside `(0, 1]`
- `burn-rate`, `multi-burn-rate`, `multi-window-multi-burn-rate` thresholds <= 0
- `multi-burn-rate` with fewer than 2 conditions per severity
- `multi-window-multi-burn-rate` with fewer than 2 tiers per severity
- `multi-window-multi-burn-rate` tier with only 1 condition (need short+long pair)
- `multi-window-multi-burn-rate` tier with non-matching thresholds across conditions
- `multi-window-multi-burn-rate` condition name that doesn't end in `-<lookbackWindow>`
- Unresolved refs anywhere in the SLO -> AlertPolicy -> AlertCondition -> AlertNotificationTarget graph
- Label names containing hyphens (Prometheus requires `[a-zA-Z_][a-zA-Z0-9_]*` - use underscores)
- Multi-value labels (more than one entry per label key)

Ref errors look like:

```
unresolved references: [unresolved ref: SLO "api-latency" references Service "missing-svc" not found]
```

and exit 1. A separate `loadSpecs` hardening in `pkg/specstore/loadSpecs` also `slog.Warn`s every file that fails to decode, so silent skips are gone - `make verify` or any `validate` run surfaces every off-shape file.

## 4. Alerting strategies

opensloctl generates Prometheus alerting rules from OpenSlo AlertCondition and AlertPolicy specs. The condition's `kind` picks one of four strategies inspired by the [Google SRE Workbook alerting on SLOs](https://sre.google/workbook/alerting-on-slos/) chapters. Pick the one that matches how aggressively you want to be paged.

Indicators can be inlined on the SLO (`spec.indicator: ...`) or referenced via `spec.indicatorRef` - referenced SLIs are resolved at generation time and treated as if they were inlined. Inline takes precedence when both are set.

> The OpenSLO SDK only models the legacy `burnrate` kind. opensloctl accepts the four kebab-case names above plus `burnrate` for back-compat; `burnrate` is mapped to `multi-window-multi-burn-rate` with a one-shot warning at generation time.

### Choosing a strategy

| Kind | SRE workbook | Used for |
|---|---|---|
| `error-rate` | §§ 1-3 | Raw SLI error rate vs an absolute threshold (e.g. 0.001 for a 99.9% SLO). One alert per severity; simplest possible setup. |
| `burn-rate` | § 4 | Single-window burn rate multiplier over the error budget. One alert per severity. |
| `multi-burn-rate` | § 5 | Two or more burn rate windows OR-ed. Each condition contributes one expression; no short/long pairing. |
| `multi-window-multi-burn-rate` | § 6 | Short+long window pairs AND-ed within a tier, OR-ed across tiers. Recommended when you can afford two windows per tier. |

### `error-rate` - SRE §§ 1-3

Compares the SLI error rate directly to an absolute threshold. Cheap to write - one condition per severity, no tiering.

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
    threshold: 0.001         # 1 - 0.999 for a 99.9% SLO
    lookbackWindow: 5m
    alertAfter: 2m
```

Generates:

```yaml
- alert: ApiLatencyErrorRate
  expr: openslo_sli_error_rate_5m{openslo_slo_name="api-latency"} >= 0.001000
  for: 2m
  labels:
    severity: page
    openslo_slo_name: api-latency
```

### `burn-rate` - SRE § 4

Single-window burn rate multiplier over the error budget. One alert per severity; the simplest meaningful burn alert.

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
- alert: CheckoutBurnRate
  expr: openslo_sli_error_rate_5m{openslo_slo_name="checkout"} / (1 - openslo_slo_objective{openslo_slo_name="checkout"}) >= 14.400000
  for: 2m
  labels:
    severity: page
    openslo_slo_name: checkout
```

### `multi-burn-rate` - SRE § 5

Two or more burn rate windows OR-ed per severity. No short/long AND pairing - each condition contributes one expression and the alert fires when any condition fires.

```yaml
# alert-condition-page-36x.yaml
- severity: page
  condition:
    kind: multi-burn-rate
    threshold: 36
    lookbackWindow: 5m
---
# alert-condition-page-6x.yaml
- severity: page
  condition:
    kind: multi-burn-rate
    threshold: 6
    lookbackWindow: 30m
```

Generates (conditions OR-ed per severity):

```yaml
- alert: ApiLatencyMultiBurnRate
  expr: |-
    (openslo_sli_error_rate_5m{openslo_slo_name="api-latency"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency"}) >= 36.000000)
    or
    (openslo_sli_error_rate_30m{openslo_slo_name="api-latency"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency"}) >= 6.000000)
  labels:
    severity: page
    openslo_slo_name: api-latency
```

Structural rule: each severity needs >= 2 conditions. If you only want one threshold, use `burn-rate` instead.

### `multi-window-multi-burn-rate` - SRE § 6

Short + long window pairs AND-ed within a tier, OR-ed across tiers. The fast/slow tiered setup is what the workbook recommends for production.

```yaml
# Tier 1: fast burn - catches a sudden spike
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

# Tier 2: slow burn - catches sustained degradation
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
- alert: ApiLatencyMultiWindowMultiBurnRate
  expr: |-
    (
    openslo_sli_error_rate_5m{openslo_slo_name="api-latency"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency"}) >= 14.400000
    and
    openslo_sli_error_rate_1h{openslo_slo_name="api-latency"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency"}) >= 14.400000)
    or
    (
    openslo_sli_error_rate_30m{openslo_slo_name="api-latency"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency"}) >= 6.000000
    and
    openslo_sli_error_rate_6h{openslo_slo_name="api-latency"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency"}) >= 6.000000)
  for: 2m
  labels:
    severity: page
    openslo_slo_name: api-latency
```

Structural rules:

- >= 2 tiers per severity (fast + slow, or any other split)
- >= 2 conditions per tier (short + long window) sharing the same threshold
- Condition names must end in `-<lookbackWindow>` so the generator can derive the tier. `page-fast-5m` and `page-fast-1h` both belong to tier `page-fast` and get AND-ed. Renaming either breaks the pairing.

Burn rate exhaustion at common SLO targets:

| Burn | 99.9% SLO (0.1% budget) | 95% SLO (5% budget) |
|---|---|---|
| 14.4x | ~2 hours | ~3.3 days |
| 6x   | ~5 hours | ~8.3 days |
| 3x   | ~10 hours | ~16.7 days |
| 1x   | ~30 days (full period) | ~30 days (full period) |

### Alert naming

Names are PascalCase with no separators: `{SloNamePascal}{KindPascal}`.

| Kind | Example alert name |
|---|---|
| `error-rate` | `AdAvailabilityErrorRate` |
| `burn-rate` | `CheckoutBurnRate` |
| `multi-burn-rate` | `ApiLatencyMultiBurnRate` |
| `multi-window-multi-burn-rate` | `AdAvailabilityMultiWindowMultiBurnRate` |

All alerts for one SLO go to a single record group `openslo-alerts-<slo-name>`. Severity is carried in the `severity` label, not the name - different severities coexist in the same group under the same alert name (Prometheus requires unique names within one group, the convention is to differentiate by `severity`).

### Field reference

| Field | Required | Where | Notes |
|---|---|---|---|
| `kind` | yes | `spec.condition.kind` | One of `error-rate`, `burn-rate`, `multi-burn-rate`, `multi-window-multi-burn-rate` (legacy `burnrate` accepted) |
| `op` | yes | `spec.condition.op` | `gte` / `gt` / `lte` / `lt`. Defaults to `gte`. Operator is non-strict by default (`gte` means `>=`); switch to `gt` if your math requires strict. |
| `threshold` | yes | `spec.condition.threshold` | Absolute (error-rate, must be in `(0, 1]`) or burn multiplier (burn-rate families, must be `> 0`) |
| `lookbackWindow` | yes | `spec.condition.lookbackWindow` | Window duration (e.g. `5m`, `1h`, `6h`) |
| `alertAfter` | no | `spec.condition.alertAfter` | Maps to Prom `for:` |
| `severity` | yes | `spec.severity` | `page`, `ticket`, or any custom string (carried in the alert label) |

## 5. Multi-dimensional SLIs

Use this when you want one SLO to drive many parallel alert series - one per value of a chosen Prometheus label.

Set two annotations on the SLO's `metadata.annotations`:

| Annotation | Required | Role |
|---|---|---|
| `multi-dimensional-sli.openslo.com/label` | yes | The Prometheus label whose value is joined into `openslo_slo_name` to produce one series per value |
| `multi-dimensional-sli.openslo.com/dimensions` | info only | Human-readable list of dimension values; not consumed by the generator |

When both annotations are set, the generator emits two layers of recording rules for the SLO:

1. **Base `_unlabeled` recordings** - each metric (`openslo_slo_info`, `openslo_slo_objective`, `openslo_slo_timewindow_days`, `openslo_slo_error_budget`, `openslo_slo_current_burn_rate`, `openslo_slo_period_burn_rate`, `openslo_slo_period_error_budget_remaining`, and every `openslo_sli_error_rate_*`) is emitted with the `_unlabeled` suffix. They carry only `openslo_slo_name` and `openslo_spec_version` labels; the chosen dimension label flows through from the underlying source query's series.
2. **Post-process `label_join` rules** - sibling rules that join the value of the chosen dimension label into `openslo_slo_name` with `-` as the separator, producing one series per dimension value.

After Prom evaluates the rules:

```
openslo_slo_current_burn_rate{openslo_slo_name="account-api-latency"}      = 0.2
openslo_slo_current_burn_rate{openslo_slo_name="checkout-api-latency"}     = 1.4
openslo_slo_current_burn_rate{openslo_slo_name="recommendation-api-latency"} = 4.7
```

`openslo_slo_current_burn_rate` here is just an example. All 11 base metrics above get the same fan-out. Same SLO target, same alert policies - but three independently firing series. Page on whichever dimension crosses 14.4x; the spec author writes the SLI once.

Working example at [`examples/multi-dim-slo/`](examples/multi-dim-slo/).

### When to use

- The underlying metric already splits by a label and you want shared target definitions across all series.
- Alert routing benefits from per-dimension firing rather than aggregate (per-caller-service paging, per-region escalation).
- You want per-dimension burn dashboards without writing one SLO per dimension.

Skip when:

- The cardinal label is unbounded (`user_id`, raw trace IDs) - recording-rule count grows linearly with cardinality and burns Prom.
- You only care about the aggregate across all series - a regular single-dim SLO is simpler.

### Trade-offs

- Recording rule count grows linearly with the cardinality of the chosen label. Bounded labels with a known max (caller services, regions, routes) work well.
- The dimension value is embedded in `openslo_slo_name` rather than as a separate label - dashboard queries must filter by prefix match (`openslo_slo_name=~"account-.*-api-latency$"`) or use the original source label.
- Alert rule names span all dimension values; severity in the `severity` label differentiates per-dimension alerts.

## 6. Dashboards and integrity rules

Three artefacts ship alongside the rules in `deploy/`.

### 6.1 `deploy/dashboards/`

Two Grafana dashboards that auto-provision via the standard sidecar pattern (`openslo-dashboards` ConfigMap, label `grafana_dashboard=1`):

- `openslo-list.json` (`OpenSLO - Manage SLOs`) - one row per SLO with columns: SLO name, Objective %, Period SLI (30d), Status (0-3 categorical), Budget Left %. Color-coded Status + Budget cells.
- `openslo-detail.json` (`OpenSLO - SLO detail`) - drilled-in view: header (name/description/target), SLI 28d timeseries + stat, Error Budget Burndown timeseries + 28d Remaining stat (percent), Error Budget Burn Rate timeseries + Current Burn Rate stat (multiplier), SLO target stat.

Both use the `datasource` and `slo` variables. Drill from list by clicking a row. Reading guide tailored for the otel-demo end-to-end flow: [`examples/oteldemo/README.md` section 2](examples/oteldemo/README.md#2-read-the-slos).

### 6.2 `deploy/mixins/`

Grafonnet source. Do not hand-edit `deploy/dashboards/*.json` - the next `make release` will overwrite them. Edit the jsonnet files and `make -C deploy/mixins release` (regenerate + sync-legacy + render integrity rules + lint-rules). Vendor dir is `tmp/grafonnet-vendor/`; populated by Grafana tooling, safe to re-bootstrap.

### 6.3 `deploy/rules/openslo-integrity-rules.yaml`

One recording rule and one alert that catch spec drift - an SLO that the generator knows about but whose `openslo_sli_error_rate_5m` series has no samples for the last 10 minutes:

```
openslo_slo_metric_missing{openslo_slo_name="..."} == 1   # offending SLOs
OpenSloSpecDrift { openslo_slo_name="..." } fire           # severity page, after 10m confirm
```

Shipped via the `deploy/rules/` ConfigMap (`make -C examples/oteldemo sync` mounts both rules + dashboards). The fix path is in the alert's `description` annotation.

## 7. Examples

Six examples ship with the repo. Five are minimal spec bundles; one (`oteldemo/`) is a full kind + Helm deployment of the OpenTelemetry Demo with our rules and dashboards.

| Directory | Strategy | Has Makefile | Has kind harness |
|---|---|---|---|
| `examples/api-latency-slo/` | `burn-rate` | no | no |
| `examples/error-budget-slo/` | `burn-rate` | no | no |
| `examples/error-rate-slo/` | `error-rate` | no | no |
| `examples/multi-burn-slo/` | `multi-burn-rate` | no | no |
| `examples/multi-dim-slo/` | `burn-rate` + multi-dim annotation | no | no |
| `examples/oteldemo/` | `multi-window-multi-burn-rate` across 11 SLOs | yes | yes |

The five minimal examples call `opensloctl` directly:

```
# check specs (CI-friendly, no files written)
opensloctl validate -f examples/api-latency-slo
opensloctl validate -f examples/error-rate-slo
opensloctl validate -f examples/multi-burn-slo
opensloctl validate -f examples/multi-dim-slo
opensloctl validate -f examples/oteldemo/specs

# generate
rm -rf output/ && mkdir output/
opensloctl generate -f examples/api-latency-slo -o output/
opensloctl generate -f examples/error-rate-slo -o output/
opensloctl generate -f examples/multi-burn-slo -o output/
opensloctl generate -f examples/multi-dim-slo -o output/
opensloctl generate -r -f examples/oteldemo/specs -o output/
ls output/
```

Each example ships a directory-specific README. The oteldemo README covers the deploy + dashboard reading flow; the others restate the strategy and file layout for their bundle.

The `oteldemo/` example is the only one with a real end-to-end harness - kind cluster, Helm chart install, ConfigMap sync, chaos flags, and counting 11 real SLOs. Start there if you want to see every generated artefact in a live cluster.

## 8. Semantic conventions

opensloctl defines a registry of SLO telemetry metrics and attributes. The registry lives in `semconv/registry/` and is used to generate `pkg/semconv/semconv_gen.go` (auto-generated, do not hand-edit).

### Attributes

| Attribute | Type | Description |
|---|---|---|
| `openslo.slo.name` | string | The SLO name |
| `openslo.spec.version` | string | OpenSLO API version of the spec |
| `openslo.service.name` | string | Service the SLO belongs to (matches the `Service` spec) |
| `openslo.alert.severity` | string | Alert severity (`page`, `ticket`, ...) on alert rules |
| `openslo.notification.target` | string | Notification target carried on alert rules (e.g. `pagerduty`, `eng-team`) |

Pass-through labels not in the registry: `openslo_slo_description` (one-line spec description folded across lines), `chaos_flag` (demo only). Both are emitted by the generator as labels on `openslo_slo_info` but defined per spec.

Deprecated attributes (still emitted as Go constants for back-compat, marked `deprecated.reason: obsoleted` in the registry):

| Attribute | Type | Reason |
|---|---|---|
| `openslo.objective.decimal` | double | Replaced by the `openslo_slo_objective` recording rule. |
| `openslo.objective.percent` | double | Replaced by the `openslo_slo_objective` recording rule. |
| `openslo.timewindow.duration` | string | Encoded in the SLI error-rate windowed recording rules. |

### Metrics

All metrics below carry `openslo.slo.name` and `openslo.spec.version` unless noted. Units are dimensionless (`1`) - alert thresholds carry the units, not the metric.

#### SLO Info

| Metric | Description |
|---|---|
| `openslo.slo.info` | Identifies the existence of an SLO. Always value 1. |
| `openslo.slo.objective` | Target SLI objective (e.g. 0.999 for 99.9%). |
| `openslo.slo.timewindow_days` | Spec `timeWindow` expressed in days. |
| `openslo.slo.error_budget` | `1 - objective`. |
| `openslo.slo.current_burn_rate` | `openslo_sli_error_rate_5m / error_budget`. |
| `openslo.slo.period_burn_rate` | `openslo_sli_error_rate_<longest> / error_budget`. Emitted when an SLO time window matches the multi-window set. |
| `openslo.slo.period_error_budget_remaining` | `clamp_min(1 - period_burn_rate, 0)`. |
| `openslo.slo.status` | Categorical health state. 0=Healthy, 1=Burning, 2=Critical, 3=Breached. See [§2.2](#22-recording-rules). |

#### SLI Error Rate

| Metric | Description |
|---|---|
| `openslo.sli.error_rate_5m` ... `_30d` | SLI error rate over each multi-window. Windows: `5m, 30m, 1h, 2h, 6h, 1d, 3d, 7d, 28d, 30d`. |

#### SLI Event Rate (RatioMetric SLIs only)

| Metric | Description |
|---|---|
| `openslo.sli.event_rate_5m` ... `_30d` | Events per second over each multi-window. Emitted only for `RatioMetric` SLIs since the spec exposes a `total` count query. Use it in dashboards to interpret error-budget burn in absolute event-volume terms. `thresholdMetric` (histogram-style) SLIs do not emit this metric. |

### Registry management

The semantic convention registry is managed with [OpenTelemetry Weaver](https://github.com/open-telemetry/weaver).

- **Registry source**: `semconv/registry/` - YAML definitions for attributes and metrics
- **Generated code**: `pkg/semconv/semconv_gen.go` - auto-generated Go constants
- **Templates**: `semconv/templates/go/` - MiniJinja templates

```
make semconv-generate             # registry YAML -> pkg/semconv/semconv_gen.go
make semconv-check                # validate registry schema
make semconv-stats                # show registry statistics
make semconv-diff BASE=<ref>      # detect breaking changes vs base ref
```

## 9. Development

### Toolchain

This project uses [mise](https://mise.jdx.dev/) to manage tool versions. You can install development dependencies via ```mise install```.

```
# developer commands
make build         # go build -o opensloctl .
make test          # go test ./...
make lint          # golangci-lint run
make tidy          # go mod tidy

# semconv management
make semconv-generate
make semconv-check
```
