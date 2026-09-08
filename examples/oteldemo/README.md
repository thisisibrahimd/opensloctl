# `oteldemo` - OpenTelemetry Demo (Astronomy Shop)

Production-scale example: 11 SLOs across the [OpenTelemetry Demo](https://github.com/open-telemetry/opentelemetry-demo) microservices, generated Prometheus rules, and Grafana dashboards for browsing every SLO. Written for SREs evaluating opensloctl who have not used OpenSLO before - read top to bottom to go from a clean machine to flipping a chaos flag and watching an SLO burn.

## Contents

- [1. Deploy the demo](#1-deploy-the-demo)
- [2. Read the SLOs](#2-read-the-slos)
- [3. Break an SLO and watch it burn](#3-break-an-slo-and-watch-it-burn)
- [4. Iterate](#4-iterate)
- [5. Reference](#5-reference)
- [6. Background](#6-background)

## 1. Deploy the demo

### 1.1 Prerequisites

- `docker`, `kind`, `helm`, `kubectl`
- `mise` provides `go`, `promtool` (or install both yourself)
- About 5 minutes and ~10 GB of disk for the chart to pull images and pods to start

### 1.2 Run

```bash
cd examples/oteldemo

make verify      # parse every spec, fail-loudly on bad YAML/SLO refs
make generate    # write examples/oteldemo/rules/<slo>-rules.yaml
make start-demo  # create kind cluster, helm install otel-demo, sync ConfigMaps
```

### 1.3 Access

The chart's `frontend` service proxies most UI traffic on `localhost:8080`. Use that single port-forward for everything except Prometheus itself.

| What | URL |
|---|---|
| Grafana (dashboards, alerting) | `http://localhost:8080/grafana/` |
| OpenTelemetry Demo UI (store, Jaeger, flagd) | `http://localhost:8080/` |
| flagd feature flags | `http://localhost:8080/feature` |
| Prometheus UI | `http://localhost:9090/` |

The two dashboards you'll use are `OpenSLO - Manage SLOs` (all SLOs, one row each) and `OpenSLO - SLO detail` (single SLO drilldown). Drill from the list by clicking any row.

### 1.4 Checkpoint

Before reading dashboards, confirm Prometheus picked up the rules:

```bash
open http://localhost:9090/rules
```

You should see `openslo-sli-recordings-` and `openslo-alerts-` groups, one per SLO. No rules means the ConfigMap sync failed or Prometheus hasn't reloaded - re-run `make -C examples/oteldemo sync`.

## 2. Read the SLOs

### 2.1 Manage SLOs dashboard

One row per SLO. Five columns:

| Column | Source metric | Meaning |
|---|---|---|
| Service Level Objective | `openslo_slo_info` | SLO name (e.g. `ad-latency`). |
| Objective % | `openslo_slo_objective * 100` | Target percentage from the spec (e.g. 95.00). |
| Period SLI | `(1 - openslo_sli_error_rate_30d) * 100` | Success ratio over the 30-day window. Always compared against the Objective column to its left. |
| Status | `openslo_slo_status` | Categorical 0-3 from current burn rate. See legend below. |
| Budget Left % | `openslo_slo_period_error_budget_remaining * 100` | Remaining error budget for the spec's `timeWindow` (28d here). 100 = untouched, 0 = exhausted. |

**Status legend** (0-3 enum, colored background):

| Value | Label | Trigger on `openslo_slo_current_burn_rate` |
|---|---|---|
| 0 | Healthy | < 1x budget pace |
| 1 | Burning | >= 1x and < 6x |
| 2 | Critical | >= 6x and < 14.4x |
| 3 | Breached | >= 14.4x |

These thresholds come from the SRE Workbook burn-rate reference points (1x = on pace, 6x = one hour of budget gone in 10 minutes, 14.4x = one hour of budget gone in 5 minutes).

**Budget Left bands**: green >= 50%, yellow >= 15%, red otherwise. A healthy dashboard shows every Status cell "Healthy" and every Budget Left cell green.

### 2.2 SLO detail dashboard

Drill into any row. The URL pins `var-slo=<name>` and `var-datasource=<uid>`. Layout:

```
+-------------------------------------------+--------+
|  ## ad-latency                            |  SLO   |  <- markdown header (name,
|  95% of GET /api/data ad fetches ...      |  95%   |     description, target)
|                                           |        |
|  **Target:** 0.95                         |        |
+-------------------------------------------+--------+
|  SLI  (28d success rate %)                | 28d SLI|
|  100% — — — — — — — — — — — — — — — —    | 99.something|
|       <target line>                       |        |
+-------------------------------------------+--------+
|  Error Budget Burndown                    | 28d Remaining|
|  (% remaining over period)               | Error Budget (%) |
+-------------------------------------------+--------+
|  Error Budget Burn Rate                   | Current Burn Rate (x) |
|  (x budget pace over windows)             | 0.0x   |
+-------------------------------------------+--------+
```

**SLO** (top-right stat): the spec's target as a percentage. e.g. an `ad-latency` spec with target 0.95 reads 95.00%.

**SLI** (left timeseries): the 28-day success rate rolling over time. Read it as: "during the last 28d, what fraction of requests succeeded?" A flat line at the target means the SLO is exactly met; a line below means the SLO is currently being missed; a line above means headroom.

**28d SLI** (right stat): the same metric as a single number (latest value over the 30-day window in the `SLO detail` time range so you can compare to the spec time window).

**Error Budget Burndown** (left timeseries): remaining budget % over the SLO time window (28d). Slope encodes burn rate: flat = no burning, downward = losing budget, hit 0 = exhausted. 100 at the window start is the ideal.

**28d Remaining Error Budget** (right stat): latest burndown value as a percentage. Color-coded green/yellow/red at 50%/15% cutoffs (same as the list dashboard).

**Error Budget Burn Rate** (left timeseries): burn rate multiplier over multiple short windows. Multiple lines render so you can spot both spike burns and sustained burns. y=1 means on budget pace (<=1x is healthy); upper lines crossing 14.4 alert the page severity (see section 3).

**Current Burn Rate** (right stat): current burn rate multiplier (no unit, raw scale). 0.5x = burning slowly, 1x = on pace, 14.4x = burning fast enough to exhaust the budget in hours.

### 2.3 Panel -> recording rule map

Every number on the dashboard is a recording rule, queryable directly in Prometheus.

| Panel | PromQL |
|---|---|
| List Objective % | `openslo_slo_objective * 100` |
| List Period SLI | `(1 - openslo_sli_error_rate_30d) * 100` |
| List Status | `openslo_slo_status` (0-3 enum) |
| List Budget Left % | `openslo_slo_period_error_budget_remaining * 100` |
| Detail header | `openslo_slo_info{openslo_slo_name="$slo"}` |
| Detail SLO stat | `openslo_slo_objective * 100` |
| Detail SLI ts | `(1 - openslo_sli_error_rate_30d) * 100` (range) |
| Detail Burndown ts | `openslo_slo_period_error_budget_remaining * 100` (range) |
| Detail Burn Rate ts | `openslo_slo_current_burn_rate` (range) |

## 3. Break an SLO and watch it burn

### 3.1 Set a chaos flag

Open `http://localhost:8080/feature` in a tab. The flagd UI lists feature flags per service. Pick one - `adServiceFailure` is the easiest to observe because it stops the ad service outright. Toggle it on and reload the store UI; ad calls start failing.

### 3.2 What to observe, in order

Within a short window after flipping the flag:

1. Detail Burn Rate timeseries spikes above 14.4 (fast tier fires fast).
2. Alerting list shows Pending, then Firing (Section 3.3).
3. List Status column for that SLO flips Healthy -> Breached.
4. List Budget Left % starts climbing down.
5. Detail Burndown timeseries slopes downward.

If you see only some of these, refresh the panel query (drill-down to detail, then back to the list row).

### 3.3 Alerts in Grafana

Prom rules surface under **Alerting > Alert rules > Data source-managed**. Filter to source = Prometheus (UID `webstore-metrics`). You'll see groups prefixed `openslo-alerts-<slo-name>`. Two severities per SLO:

- `page`: tier fast (5m AND 1h at 14.4x) OR tier slow (30m AND 6h at 6x). Fires fast.
- `ticket`: tier fast (2h AND 1d at 3x) OR tier slow (6h AND 3d at 1x). Fires on sustained, slower burn.

The April 2016 SRE Workbook chapter on alerting on SLOs is what tuned these numbers.

### 3.4 Recovery

Disable the flag. Status stays "Breached" for as long as the longest window in the worst tier is still elevated - roughly an hour for `page-fast-1h`, six hours for `page-slow-6h`. The status gauge, burndown, and budget stat all lag the underlying metric by the largest window of their respective recording rules.

### 3.5 Chaos flag to SLO

| flagd flag | SLO |
|---|---|
| `adServiceFailure` | `ad-availability` |
| `cartServiceFailure` | `cart-availability` |
| `paymentServiceUnreachable` | `payment-unreachable` |
| `recommendationCacheFailure` | `recommendation-availability` |
| `imageSlowLoad` | `image-loading-latency` |
| `kafkaQueueProblems` | `order-processing-latency` |
| `emailMemoryLeak` | `post-order-email-availability`, `post-order-email-latency` |
| `productCatalogFailure` | `product-catalog-availability` |
| `adServiceHighCpu` | `ad-latency` |

Note: the spec label `chaos_flag` matches the intuitive name, not the flagd JSON canonical name. If you're chasing a flag via the OpenSLO spec, look for the friendly string above; if you're chasing it via flagd's API, the JSON keys are different (e.g. `adFailure` not `adServiceFailure`). Both forms exist intentionally.

## 4. Iterate

Quick recipes for the three edits you'll likely make first.

### 4.1 Change an SLO spec

```bash
$EDITOR examples/oteldemo/specs/<slo>.yaml     # edit target, description, indicator source
make -C examples/oteldemo verify                # spec refs still resolve
make -C examples/oteldemo generate              # rewrites rules/<slo>-rules.yaml
make -C examples/oteldemo sync                  # push ConfigMap, reload Prometheus
```

### 4.2 Change the colletor bucket list

```bash
$EDITOR examples/oteldemo/kind/values.yaml       # extend the explicit bucket list
make -C examples/oteldemo start-demo            # idempotent - re-renders Helm chart
# then
make -C examples/oteldemo sync                  # rules + dashboards
```

### 4.3 Change a dashboard

The dashboard JSON is generated from `deploy/mixins/<name>.jsonnet`, not edited directly.

```bash
$EDITOR deploy/mixins/<name>.jsonnet
make -C deploy/mixins release    # generate + sync-legacy + rules + lint-rules
make -C examples/oteldemo sync   # ConfigMap push
```

### 4.4 Teardown

```bash
make -C examples/oteldemo stop-demo   # deletes the kind cluster
make -C examples/oteldemo clean       # rm -rf rules/
```

## 5. Reference

### 5.1 SLO inventory

11 SLOs, one per spec file under `specs/`. Latency SLOs use `ratioMetric` over classic-bucket `traces_span_metrics_*` rather than `thresholdMetric` - see [Section 6.2](#62-indicator-shape-latency-slos).

| SLO | Service | Type | Target | Bucket / label | Chaos flag |
|---|---|---|---|---|---|
| `ad-availability` | ad | availability | 0.99 | - | `adServiceFailure` |
| `ad-latency` | ad | latency | 0.95 | `le="2000"` | `adServiceHighCpu` |
| `cart-availability` | cart | availability | 0.99 | - | `cartServiceFailure` |
| `frontend-availability` | frontend | availability | 0.95 | - | - |
| `image-loading-latency` | image-provider | latency | 0.95 | `le="2000"` | `imageSlowLoad` |
| `order-processing-latency` | checkout | latency | 0.95 | `le="60000"` | `kafkaQueueProblems` |
| `payment-unreachable` | checkout | availability | 0.99 | - | `paymentServiceUnreachable` |
| `post-order-email-availability` | email | availability | 0.95 | - | `emailMemoryLeak` |
| `post-order-email-latency` | email | latency | 0.95 | `le="30000"` | `emailMemoryLeak` |
| `product-catalog-availability` | productcatalogservice | availability | 0.95 | - | `productCatalogFailure` |
| `recommendation-availability` | recommendationservice | availability | 0.95 | - | `recommendationCacheFailure` |

### 5.2 Layout

```
oteldemo/
├── kind/                              # kind cluster + Helm harness
│   ├── kind-config.yaml               # cluster spec
│   ├── values.yaml                    # Helm chart overrides (bucket list,
│   │                                  #   span_metrics connector wiring,
│   │                                  #   enable-feature flags)
│   ├── setup.sh                       # cluster create + helm install +
│   │                                  #   initial ConfigMap sync
│   ├── teardown.sh                    # delete kind cluster
│   └── sync.sh                        # re-apply ConfigMaps after make generate
├── rules/                             # generated YAML (gitignored after clean)
├── specs/                             # 11 SLOs + 3 helper files
│   ├── services.yaml                  # inventory of demo services
│   ├── alert-conditions.yaml          # 8 reusable AlertConditions
│   ├── alert-policies.yaml            # 8 AlertPolicies wrapping them
│   ├── notification-target-engineers.yaml
│   ├── <slo>-availability.yaml        # 7 availability SLOs
│   └── <slo>-latency.yaml             # 4 latency SLOs
├── Makefile                           # verify / generate / start-demo /
│                                      #   stop-demo / sync / clean
└── README.md                          # this file
```

### 5.3 Make targets

| Target | Effect |
|---|---|
| `make verify` | Run `go run . load` over `specs/`. Validates YAML + specstore contracts. |
| `make generate` | Run `go run . generate` over `specs/`, write `rules/`. Calls `promtool check rules`. |
| `make lint-rules` | `promtool check rules` on every `rules/*.yaml`. |
| `make start-demo` | `kind/setup.sh` - creates cluster, installs chart, syncs ConfigMaps. Idempotent on an existing cluster. |
| `make stop-demo` | `kind/teardown.sh` - deletes the cluster. |
| `make sync` | `kind/sync.sh` - replaces `openslo-rules` and `openslo-dashboards` ConfigMaps (delete + create to avoid the 256 KiB annotation ceiling), reloads Prometheus. |
| `make clean` | Removes `rules/`. |

## 6. Background

These sections explain *why* the dashboards look the way they do. Skim them if you're debugging; skip if you're just deploying.

### 6.1 Alerting strategy

Each SLO references 8 AlertConditions - two severities (page, ticket), each with two paired tiers of short+long windows:

| Tier | windows | threshold | meaning |
|---|---|---|---|
| page fast | 5m AND 1h | 14.4x | short and long agree: hard spike |
| page slow | 30m AND 6h | 6x | short and long agree: sustained moderate burn |
| ticket fast | 2h AND 1d | 3x | spike worth a ticket |
| ticket slow | 6h AND 3d | 1x | the SLO is burning at exactly the budget pace |

Within a tier conditions AND. Across tiers within a severity conditions OR. So `page` fires on `(fast-5m AND fast-1h) OR (slow-30m AND slow-6h)`.

Condition **name suffix `-<lookbackWindow>` is mandatory** - the generator strips it to derive the tier. `burnrate-page-fast-5m` and `burnrate-page-fast-1h` both belong to tier `burnrate-page-fast` and get AND-ed. Renaming either breaks the pairing.

Conditions and policies live once in `alert-conditions.yaml` and `alert-policies.yaml`. Each SLO references the same 8 conditions - no per-SLO alert duplication.

### 6.2 Indicator shape (latency SLOs)

The 4 latency SLOs use `ratioMetric(counter=true)`:

```
good   = sum(rate(traces_span_metrics_duration_milliseconds_bucket{..., le="<value_ms>"}[<window>]))
total  = sum(rate(traces_span_metrics_calls_total{...}[<window>]))
error  = 1 - good / total
```

The `le=` boundary must exist in the connector's bucket list. The otel-demo chart defaults miss the longer envelopes, so `kind/values.yaml` extends the bucket list to include `30s` and `60s` for `post-order-email-latency` (`le="30000"`) and `order-processing-latency` (`le="60000"`).

Why ratio over `histogram_quantile(...) > <threshold>`: Prom 3.x filter semantics collapses raw scalar comparisons to filter semantics - `histogram_quantile(...) > 2000` returns the quantile unchanged, not a 0/1 series, so it can't drive a ratio SLI over a 30-day window without an explicit `bool` modifier in the comparison. Native exponential histograms have no `le="..."` series, so the per-threshold fraction can't be expressed at all. Classic-bucket ratio is the shape that reads as a continuous value across rolling windows.

### 6.3 Why kind + Helm

Docker compose bind mounts struggle when the same target dir needs both upstream and custom content, or has other mounts layering in. Kubernetes ConfigMaps sidestep both. The upstream otel-demo chart ships Prometheus and Grafana with their sidecar pattern enabled, so we layer our ConfigMaps on without touching the chart.

### 6.4 When to use this kind

`multi-window-multi-burn-rate` is the recommended SRE Workbook pattern (§ 6) for any SLO you care about. It catches sudden spikes (fast tier, short window, high multiplier) and sustained moderate burns (slow tier, long window, lower multiplier) while rejecting noise (the AND within a tier). Trade down to [`multi-burn-rate`](../multi-burn-slo/README.md) if you don't want the AND pairing; trade down further to [`burn-rate`](../api-latency-slo/README.md) for the simplest possible single-window.
