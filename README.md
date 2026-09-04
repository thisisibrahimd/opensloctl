# opensloctl

Generate Prometheus recording rules and alerting rules from OpenSlo specs.

## Table of Contents

- [Installation](#installation)
- [Development](#development)
- [Commands](#commands)
- [Usage](#usage)
  - [Recording Rules](#recording-rules-slo-name-recording-rulesyaml)
  - [Alert Rules](#alert-rules-slo-name-alert-rulesyaml)
- [Burn Rate Alerts](#burn-rate-alerts)
  - [How It Works](#how-it-works-1)
  - [Example](#example-api-latency-slo-with-page--ticket-alerts)
  - [Why Four Alert Conditions](#why-four-alert-conditions)
  - [Creating AlertConditions](#creating-alertconditions)
  - [Creating AlertPolicies](#creating-alertpolicies)
  - [Linking to SLOs](#linking-to-slos)
  - [Validation](#validation)
  - [Run the Examples](#run-the-examples)
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
go run . generate -f <file> -o <dir>  # generate Prometheus recording rules
make semconv-generate             # regenerate semconv_gen.go from registry
make semconv-check                # validate registry schema
make lint                         # run golangci-lint
make test                         # run go test ./...
```

## Usage

opensloctl reads OpenSlo SLO, SLI, AlertCondition, and AlertPolicy specs and generates two types of Prometheus rule files:

### Recording Rules (`<slo-name>-recording-rules.yaml`)

For each SLO, opensloctl generates Prometheus recording rules that:

1. **Expose SLO metadata** — `openslo_slo_info`, `openslo_slo_objective`, `openslo_slo_timewindow_days`, `openslo_slo_error_budget`
2. **Pre-compute SLI error rates** — `openslo_sli_error_rate_5m`, `_30m`, `_1h`, `_2h`, `_6h`, `_1d`, `_3d`, `_7d`, `_28d`, `_30d`

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
```

Multiline queries are preserved using YAML block scalars (`|`):

```yaml
    - record: openslo_sli_error_rate_5m
      expr: |
        histogram_quantile(0.99,
          sum(rate(http_request_duration_seconds_bucket{job="api"}[5m])) by (le))
```

### Alert Rules (`<slo-name>-alert-rules.yaml`)

When an SLO references AlertPolicies with burn rate conditions, opensloctl generates Prometheus alerting rules. Conditions with the same severity are OR-ed together into a single alert rule:

```yaml
groups:
  - name: openslo-burnrate-alerts-api-latency-slo
    rules:
    - alert: OpenSLO_Page_BurnRate_api_latency_slo
      expr: |-
        openslo_sli_error_rate5m{openslo_slo_name="api-latency-slo"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency-slo"}) gte 14.4
        or
        openslo_sli_error_rate30m{openslo_slo_name="api-latency-slo"} / (1 - openslo_slo_objective{openslo_slo_name="api-latency-slo"}) gte 6.0
      for: 2m
      labels:
        severity: page
        openslo_slo_name: api-latency-slo
```

## Burn Rate Alerts

opensloctl supports generating Prometheus alerting rules from OpenSlo AlertCondition and AlertPolicy specs. Follow the [Google SRE Workbook multi-window multi-burn-rate](https://sre.google/workbook/alerting-on-slos/#6-multiwindow-multi-burn-rate-alerts) pattern.

### How It Works

1. Define **AlertConditions** with burn rate thresholds and windows
2. Group them into **AlertPolicies** (one condition per policy)
3. Reference policies from your **SLO** via `spec.alertPolicies[]`
4. All refs must resolve — validation runs on load

### Example: API Latency SLO with Page + Ticket Alerts

```
examples/api-latency-slo/
├── service.yaml
├── datasource.yaml
├── sli.yaml                    # thresholdMetric: P99 latency
├── alert-condition-page.yaml   # 14.4x burn rate, 5m window
├── alert-condition-ticket.yaml # 3x burn rate, 2h window
├── alert-policy-page.yaml      # page → pagerduty
├── alert-policy-ticket.yaml    # ticket → slack
├── notification-target-*.yaml
└── slo.yaml                    # references both policies
```

### Why Four Alert Conditions

The Google SRE Workbook recommends **four AlertConditions** per SLO — two for page severity and two for ticket severity. Each condition represents a different burn rate window, and conditions within the same severity are **OR-ed** together.

```
Page alerts fire if EITHER condition is true:
  (14.4x burn rate over 5m)  OR  (6x burn rate over 30m)

Ticket alerts fire if EITHER condition is true:
  (3x burn rate over 2h)     OR  (1x burn rate over 6h)
```

This is the **multi-window multi-burn-rate** pattern. You need two windows per severity to:

1. **Catch sudden spikes** — the fast window (5m at 14.4x) fires immediately when error rate spikes hard
2. **Catch sustained degradation** — the slow window (30m at 6x) fires when error rate is moderately elevated for longer
3. **Reduce false positives** — both windows must agree on the burn rate within their respective timeframes, but the OR means you get alerted if either window detects the problem

The burn rate values are derived from the error budget math. For a 99.9% SLO (0.1% error budget):
- **14.4x** burns through the 30-day budget in ~2 hours
- **6x** burns through the 30-day budget in ~5 hours
- **3x** burns through the 30-day budget in ~10 hours
- **1x** burns through the 30-day budget in ~30 days (full budget exhaustion)

### Creating AlertConditions

Define all four conditions, one per file:

```yaml
# alert-condition-page-14x.yaml — fast page alert
apiVersion: openslo/v1
kind: AlertCondition
metadata:
  name: api-latency-page-14x
spec:
  severity: page
  description: Page on-call when API latency burn rate spikes hard
  condition:
    kind: burnrate          # only "burnrate" is supported
    op: gte                 # gte, lte, gt, lt
    threshold: 14.4         # burn rate multiplier
    lookbackWindow: 5m      # evaluation window
    alertAfter: 2m          # Prometheus "for" duration (optional)
```

```yaml
# alert-condition-page-6x.yaml — slow page alert
apiVersion: openslo/v1
kind: AlertCondition
metadata:
  name: api-latency-page-6x
spec:
  severity: page
  condition:
    kind: burnrate
    threshold: 6
    lookbackWindow: 30m
    alertAfter: 5m
```

```yaml
# alert-condition-ticket-3x.yaml — fast ticket alert
apiVersion: openslo/v1
kind: AlertCondition
metadata:
  name: api-latency-ticket-3x
spec:
  severity: ticket
  condition:
    kind: burnrate
    threshold: 3
    lookbackWindow: 2h
    alertAfter: 15m
```

```yaml
# alert-condition-ticket-1x.yaml — slow ticket alert
apiVersion: openslo/v1
kind: AlertCondition
metadata:
  name: api-latency-ticket-1x
spec:
  severity: ticket
  condition:
    kind: burnrate
    threshold: 1
    lookbackWindow: 6h
    alertAfter: 30m
```

### How Conditions Are OR-ed

Conditions with the same `severity` are grouped together and combined with **OR** logic in the generated Prometheus alerting rules:

```yaml
# Generated Prometheus alert rule for page severity
- alert: openslo_slo_burn_rate
  expr: |
    (
      openslo_sli_error_rate_5m{openslo_slo_name="api-latency-slo"} > 14.4 * openslo_slo_error_budget{openslo_slo_name="api-latency-slo"}
    )
    or
    (
      openslo_sli_error_rate_30m{openslo_slo_name="api-latency-slo"} > 6 * openslo_slo_error_budget{openslo_slo_name="api-latency-slo"}
    )
  for: 2m
  labels:
    openslo_slo_name: api-latency-slo
    severity: page
```

Each severity gets its own alert rule. The page rule ORs both page conditions together. The ticket rule ORs both ticket conditions together. This means:
- If **either** the 5m or 30m window exceeds its threshold → page fires
- If **either** the 2h or 6h window exceeds its threshold → ticket fires

### Creating AlertPolicies

The OpenSlo SDK enforces exactly 1 condition per AlertPolicy. So you need 4 policies — one per condition. The **OR-ing happens at the Prometheus alert rule level**, not in the OpenSlo spec.

```yaml
# alert-policy-page-14x.yaml
apiVersion: openslo/v1
kind: AlertPolicy
metadata:
  name: api-latency-page-14x-alert
spec:
  description: Page alert for 14.4x burn rate
  alertWhenBreaching: true
  conditions:
    - conditionRef: api-latency-page-14x
  notificationTargets:
    - targetRef: oncall-pagerduty
```

Repeat for the other three conditions (page-6x, ticket-3x, ticket-1x). Each gets its own policy file.

### How Conditions Are OR-ed

The generator groups conditions by `severity` and creates **one Prometheus alert rule per severity** with OR logic:

```yaml
# Generated: page alert rule (ORs page-14x and page-6x)
- alert: openslo_slo_burn_rate
  expr: |
    (
      openslo_sli_error_rate_5m{openslo_slo_name="api-latency-slo"} > 14.4 * openslo_slo_error_budget{openslo_slo_name="api-latency-slo"}
    )
    or
    (
      openslo_sli_error_rate_30m{openslo_slo_name="api-latency-slo"} > 6 * openslo_slo_error_budget{openslo_slo_name="api-latency-slo"}
    )
  for: 2m
  labels:
    openslo_slo_name: api-latency-slo
    severity: page

# Generated: ticket alert rule (ORs ticket-3x and ticket-1x)
- alert: openslo_slo_burn_rate
  expr: |
    (
      openslo_sli_error_rate_2h{openslo_slo_name="api-latency-slo"} > 3 * openslo_slo_error_budget{openslo_slo_name="api-latency-slo"}
    )
    or
    (
      openslo_sli_error_rate_6h{openslo_slo_name="api-latency-slo"} > 1 * openslo_slo_error_budget{openslo_slo_name="api-latency-slo"}
    )
  for: 15m
  labels:
    openslo_slo_name: api-latency-slo
    severity: ticket
```

Page fires if **either** 5m@14.4x **or** 30m@6x fires. Ticket fires if **either** 2h@3x **or** 6h@1x fires.

Notification targets:

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

Reference alert policies from your SLO:

```yaml
apiVersion: openslo/v1
kind: SLO
metadata:
  name: api-latency-slo
spec:
  service: api-gateway
  indicatorRef: api-latency-p99
  budgetingMethod: Occurrences
  timeWindow:
    - duration: 30d
      isRolling: true
  objectives:
    - displayName: "P99 latency < 500ms"
      target: 0.999
      op: lte
      value: 500
  alertPolicies:
    - alertPolicyRef: api-latency-page-alert
    - alertPolicyRef: api-latency-ticket-alert
```

### Inline Conditions and Targets

You can inline conditions and notification targets directly in the AlertPolicy instead of using refs:

```yaml
apiVersion: openslo/v1
kind: AlertPolicy
metadata:
  name: api-latency-page-alert
spec:
  description: Page alert for API latency burn rate
  alertWhenBreaching: true
  conditions:
    - kind: AlertCondition
      metadata:
        name: api-latency-page
      spec:
        severity: page
        condition:
          kind: burnrate
          threshold: 14.4
          lookbackWindow: 5m
          alertAfter: 2m
  notificationTargets:
    - kind: AlertNotificationTarget
      metadata:
        name: oncall-pagerduty
      spec:
        target: pagerduty
```

### Validation

All references are validated on load. If any ref cannot be resolved, you get an error listing all missing refs:

```
unresolved references: [unresolved ref: SLO "api-latency-slo" references Service "missing-svc" not found]
```

The CLI also validates:
- Each spec passes SDK validation (required fields, value ranges)
- AlertCondition `kind` must be `burnrate`
- AlertPolicy must have exactly 1 condition

### Run the Examples

```bash
# Load and validate the API latency SLO
opensloctl load -r -f examples/api-latency-slo

# Load and validate the checkout availability SLO
opensloctl load -r -f examples/error-budget-slo

# Generate recording rules + alert rules
opensloctl generate -r -f examples/api-latency-slo -o output/
ls output/
# api-latency-slo-recording-rules.yaml
# api-latency-slo-alert-rules.yaml
```

## Semantic Conventions

opensloctl defines a registry of metrics and attributes for SLO telemetry. The registry lives in `semconv/registry/` and is used to generate `pkg/semconv/semconv_gen.go`.

### Attributes

| Attribute | Type | Description |
|---|---|---|
| `openslo.slo.name` | string | The name of the SLO as defined in the OpenSlo spec. |
| `openslo.spec.version` | string | The OpenSLO API version of the SLO spec. |

### Metrics

#### SLO Info

| Metric | Type | Unit | Description |
|---|---|---|---|
| `openslo.slo.info` | gauge | 1 | Identifies the existence of an SLO. Always has value 1. |
| `openslo.slo.objective` | gauge | 1 | The target SLI objective (e.g., 0.999 for 99.9% availability). |
| `openslo.slo.timewindow_days` | gauge | 1 | The SLO time window duration expressed as a number of days. |
| `openslo.slo.error_budget` | gauge | 1 | The error budget calculated as 1 minus the objective. |

All SLO info metrics carry `openslo.slo.name` and `openslo.spec.version` labels.

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

All error rate metrics carry `openslo.slo.name` and `openslo.spec.version` labels.

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
meter.Float64ObservableGauge(semconv.METRIC_OPENSLO_SLO_INFO)
```
