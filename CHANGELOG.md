# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> Active development. Pin a specific release if you are relying on it.

## [v0.2.0]

> Generator v0.2.0 reaches parity with the v0.1.8-dev contract: the four
> alerting strategies (`error-rate`, `burn-rate`, `multi-burn-rate`,
> `multi-window-multi-burn-rate`) emit recorded SLO metadata plus
> severity-suffixed alerts; Grafana dashboards for the oteldemo
> bundle auto-provision; the otel-demo harness now uses kind + Helm in
> place of the previous docker-compose layout.

### Breaking changes

These are the breaking items integrators should plan around when moving
from <=v0.1.0 to v0.2.0:

- **`specstore/loadSpecs` fails loudly on undelivered files.** Every
  shadowed load inside the SDK decoder that previously dropped a file
  to stderr now returns it as a structured `slog.Warn` and bubbles up
  into a wrapped error from `GetSpecs`. Runners that relied on stray
  YAML being silently ignored start failing CI. (`7f3b4e8`)
- **`openslo_slo_info` and `openslo_sli_*` series gain an
  `openslo_slo_description` label.** The recording rule text carries
  each spec's `spec.description` folded to one line. Dashboards matching
  on exact label sets will see new series in addition to existing
  ones. (`feat(generator, semconv): emit openslo_slo_description label`)
- **Alert rules now append the severity PascalCase to the alert
  name.** `AdAvailability...` (severity not encoded in name) is
  replaced by `AdAvailability...Page` and `AdAvailability...Ticket`.
  The `severity` label is unchanged, so Prometheus and Alertmanager
  routing still match. Alertname-based lookups in alert dashboards
  and runbooks need to be updated.
- **`examples/oteldemo/deploy/` docker-compose harness removed.**
  Replaced by `examples/oteldemo/kind/` which runs the upstream
  `open-telemetry/opentelemetry-demo` Helm chart on a single-node
  `kind` cluster. Top-level `make start-demo` target is gone; the
  per-example `make -C examples/oteldemo start-demo` is the single
  lifecycle entrypoint.

### Added

- **Generator templates for the four SRE strategies.** Single unified
  `<slo-name>-rules.yaml` per SLO covering recording rules (info,
  objective, timewindow, error budget, windowed `sli_error_rate_*`,
  `sli_event_rate_*` for RatioMetric SLIs, current and period burn
  rate, period error budget remaining, categorical status gauge) and
  when alert policies exist - an `openslo-alerts-<slo-name>` group
  inside the same file.
- **Multi-window-multi-burn-rate emitting shared reusable
  AlertCondition definitions.** Eight conditions in oteldemo
  `alert-conditions.yaml` cover page-fast (5m AND 1h at 14.4x),
  page-slow (30m AND 6h at 6x), ticket-fast (2h AND 1d at 3x),
  ticket-slow (6h AND 3d at 1x). Per-SLO AlertPolicies reference them
  - no per-SLO alert duplication.
- **Categorical status gauge `openslo_slo_status`** with overridable
  thresholds via `threshold.status.openslo.com/{warning,critical,breached}`
  annotations. Defaults `1/6/14.4` follow the Google SRE Workbook.
- **Multi-dimensional SLI annotations** via
  `multi-dimensional-sli.openslo.com/{label,dimensions}`. The
  generator emits `_unlabeled` rule variants followed by a `label_join`
  post-process group; one SLO becomes one series per value of the
  chosen PromQL label.
- **Grafana dashboards with grafonnet mixins source of truth:
  `deploy/mixins/*.jsonnet`**. Makefile `generate` + `release`
  regenerates `deploy/dashboards/*.json`. See "Dashboards" below.
- **Spec-drift integrity rule and alert.**
  `openslo_slo_metric_missing` recording rule + `OpenSloSpecDrift` page
  alert, rendered from `deploy/mixins/rules/openslo-integrity.jsonnet`
  to `deploy/rules/openslo-integrity-rules.yaml`. Caught
  inconsistencies where the SDK has registered an SLO but the
  underlying SLI query is producing no samples (typo in indicator
  spec, denied Prom access, or a stuck recording rule).
- **`examples/oteldemo` kind + Helm harness.** Replaces the prior
  docker-compose harness. `setup.sh` creates a single-node cluster
  and installs the upstream Helm chart; `sync.sh` re-applies
  ConfigMaps (delete+create to avoid the 256 KiB
  `kubectl.kubernetes.io/last-applied-configuration` annotation
  ceiling); `teardown.sh` deletes the cluster and release.
- **otel-demo Mixin customisations.**
  `examples/oteldemo/kind/values.yaml` extends the otel-collector
  histogram bucket list to include 30s and 60s for
  `post-order-email-latency` (`le="30000"`) and `order-processing-latency`
  (`le="60000"`). The chart's defaults miss these envelopes, so
  ratio SLIs whose boundary sits above the chart's max bucket would
  silently read `+Inf`.
- **`pkg/semconv` Weaver-managed registry.** New
  `openslo.slo.description` semconv attribute and `openslo.alert.severity`
  / `openslo.notification.target` carriers. Deprecated
  `openslo.objective.decimal`, `openslo.objective.percent`,
  `openslo.timewindow.duration` retained as Go constants for one
  cycle, marked `deprecated.reason: obsoleted`.
- **`TestStatusRuleUsesBoolModifier` regression test** locks in the
  Prom 3.x `bool` modifier on every status block comparison
  (3 `>= bool`, 2 `<  bool`, no `bool` in alert blocks).
- **`examples/multi-dim-slo/` example** with its dedicated README,
  walker-friendly spec fixtures, and single README.

### Changed

- **`openslo_slo_info` label set**: now includes
  `openslo_slo_description` (string, folded from `spec.description`,
  capped at 200 chars, escape-quote/backslash). The recording rule
  text is invariantly emitted so downstream Grafana `${openslo_slo_description}`
  substitutions always resolve.
- **Alert naming convention**: `{SloNamePascal}{KindPascal}` now
  suffixed with `{SeverityPascal}` per severity, line up with the
  uniqueness check inside a single rule group. Same diff per severity,
  alert block text unchanged.
- **`examples/oteldemo/specs/` inventory pruned from 12 to 11**
  with the otel-demo natural baseline:

  | `ad-availability.yaml`             | kept, slope 0.99/0.95 |
  | `ad-latency.yaml`                  | new, ratio `le="2000"` |
  | `cart-availability.yaml`           | kept                  |
  | `frontend-availability.yaml`       | kept                  |
  | `image-loading-latency.yaml`       | new, ratio `le="2000"` on `image-provider` source |
  | `order-processing-latency.yaml`    | new, ratio `le="60000"` for checkout fan-out |
  | `payment-unreachable.yaml`        | kept, narrower than pre-v0.2.0 shape; trips on `paymentUnreachable` |
  | `post-order-email-availability.yaml` | new, email SERVER `POST /send_order_confirmation`, trips on `emailMemoryLeak` |
  | `post-order-email-latency.yaml`    | new, `le="30000"` |
  | `product-catalog-availability.yaml` | kept                |
  | `recommendation-availability.yaml` | kept                 |

- **Path Y migration**: 6 SLOs moved to frontend HTTP SERVER route
  span metrics (`service_name="frontend"`, `span_kind="SPAN_KIND_SERVER"`,
  `span_name=<route>`). Removes flagd-client noise (EventStream
  reconnects, ResolveBoolean failures) that previously inflated error
  rates by 7-9% on backend-service span metrics.
- **`chaos_flag` label drift documented in `AGENTS.md`**: SLO specs
  use `adServiceFailure` / `cartServiceFailure` /
  `paymentServiceUnreachable` / `recommendationServiceCacheFailure`
  while flagd JSON canonical is `adFailure` / `cartFailure` /
  `paymentUnreachable` / `recommendationCacheFailure`. Mature drift
  on purpose for the demo. Aligning the canonical form remains future
  work.
- **Target locked per SLO**: availability at 0.95 by default;
  `payment-unreachable` and `order-processing-latency` at 0.99 because
  their natural-baseline burn is below 1x only when aimed at this
  tightest tier.
- **Status threshold annotations**: now strictly ascending and
  positive at parse time; non-numeric values silently fall back to
  defaults so spec-author scratch notes don't break generation.

### Fixed

- **Generator operator-precedence bug** in
  `internal/generator/prometheusgenerator/prometheus.go`. PromQL parses
  left-to-right at equal operator precedence, so the previous template
  output `(1 - good) / total` rather than the intended
  `1 - (good / total)`. For SLOs with healthy success rates the emitted
  `openslo_sli_error_rate_*` was inflated roughly `1 / success_rate`
  times larger, corrupting the downstream current-burn-rate and the
  categorical status gauge.
- **Status gauge Prom 3.x compatibility**: every comparison in the
  status recording rule now uses the `bool` modifier. Without it,
  Prom 3.x comparison filters preserve the LHS series unchanged, so
  `(burn_rate >= 14.4) * 3` returned the raw burn rate (e.g. `57.3`)
  instead of `3`, breaking the dashboard's
  `0/1/2/3 -> Healthy/Burning/Critical/Breached` mapping.
- **`pkg/specstore/loadSpecs` silent-skip bug.** Decoder errors are now
  logged at warn and surfaced as a non-nil error from `GetSpecs`.
  Stray top-level fields on an OpenSlo spec no longer vanish from
  the generator run.
- **Specstore testdata restructure** to support the fail-loud
  fix: the intentionally invalid fixtures moved from `testdata/`
  into `testdata/invalid/`, away from recursive walks.

### Removed

- `examples/oteldemo/deploy/` directory (compose harness).
- `examples/oteldemo/openslo-dashboards.yaml` (legacy direct-mount
  dashboard format, replaced by grafonnet mixins).
- `examples/oteldemo/dashboards/` orphaned dashboard stash.
- Top-level `Makefile` `start-demo` target (the per-example
  `make -C examples/oteldemo start-demo` is the single entry-point).

### Deprecated

- `openslo.objective.decimal` / `openslo.objective.percent` /
  `openslo.timewindow.duration` semconv attributes. Still emitted as
  Go constants for one cycle. Removal is planned for the next major
  version (>= v1.0.0).

### Dashboards

Two production-ready Grafana dashboards ship under `deploy/dashboards/`:

- `openslo-list.json` (`OpenSLO - Manage SLOs`): one row per SLO with
  Objective %, Period SLI (30d), categorical Status (0/1/2/3 mapped
  to Healthy/Burning/Critical/Breached), and Budget Left % (color
  bands 50/15). Drill-down link to per-SLO detail dashboard.
- `openslo-detail.json` (`OpenSLO - SLO detail`): markdown header
  (name, description, target), SLI timeseries (28d SLI), Error
  Budget Burndown (28d), Error Budget Burn Rate (multi-window).
  Each row has its corresponding stat panel.

Variables: `datasource` (Prometheus picker, `pluginId: prometheus`,
locked lowercase) and `slo` (label_values over `openslo_slo_name`).
Two hidden helpers hidden behind `hide: 2` are `description` (regex
extracts `openslo_slo_description` label) and `target` (regex
extracts the value of `openslo_slo_objective`) - used to render the
header markdown without cluttering the picker.

Source of truth: `deploy/mixins/*.jsonnet` (grafonnet v13 + sprig).
Rendered by `make -C deploy/mixins generate`, committed as JSON via
`make sync-legacy`.

### Security

No security-sensitive changes in this iteration. The CLI does not
perform network I/O outside of `go build`/`go test`/`go run`, and
demo-harness scripts run against a local `kind` cluster the user
controls.

[Unreleased]: https://github.com/thisisibrahimd/opensloctl/compare/v0.2.0...HEAD
[v0.2.0]: https://github.com/thisisibrahimd/opensloctl/compare/v0.1.0...v0.2.0
