# AGENTS.md

> Worktree branch: `demo-slos`. Verify `git status` before committing - main branch layout may differ slightly.

## Commands

```
make build                        # go build -o opensloctl .
make lint                         # golangci-lint run
make test                         # go test ./...
make tidy                         # go mod tidy
make load FILE=<file>             # parse and print OpenSlo specs
make validate FILE=<file>         # validate OpenSlo specs without writing files
make generate FILE=<f> OUTPUT=<d> # generate Prometheus rules (<slo-name>-rules.yaml)
```

Via `go run` (supports `-r` recursive flag, Makefile targets do not):
```
go run . load -f <file> [-r]
go run . validate -f <file> [-r]
go run . generate -f <file> -o <dir> [-r]
```

Semconv registry (Weaver):
```
make semconv-generate             # registry YAML → pkg/semconv/semconv_gen.go
make semconv-check                # validate registry schema
make semconv-stats                # show registry statistics
make semconv-json                 # output registry JSON schema
make semconv-diff BASE=<ref>      # detect breaking changes vs base ref
```

## Architecture

- `main.go` → `cmd.Execute()` - single entrypoint
- CLI: cobra-based, three subcommands: `load`, `validate`, `generate`
  - All accept `-f` (filename, repeatable) and `-r` (recursive directory scan)
  - `generate` also requires `-o` (output directory)
  - `validate` runs full validation (load-time + generator-side) without writing files
- `pkg/specstore/loader.go` - loads YAML files via `openslosdk.Decode`, sorts into typed `OpenSloSpecs` struct
- `internal/generator/generator.go` - `Generator` interface
- `internal/generator/prometheusgenerator/` - generates Prometheus rules YAML from SLO specs using Go templates + sprig (embedded via `//go:embed`). One unified output file per SLO: `<slo-name>-rules.yaml` (covering recording rules and, if alert policies are referenced, alert rules via an `openslo-alerts-<slo-name>` group inside the same file).
- `internal/feature/feature.go` - feature flags for multi-dimensional SLI annotations
- `pkg/semconv/semconv_gen.go` - **auto-generated** from semconv registry (do not edit manually)
- `pkg/util/file.go` - recursive YAML/YML file discovery
- `semconv/registry/` - OpenTelemetry Weaver registry YAML (metrics + attributes)
- `semconv/templates/go/` - MiniJinja templates for semconv codegen
- `examples/<kind>-slo/specs/` - OpenSLO spec sets (input)
- `examples/<kind>-slo/rules/` - generated Prometheus rule files (output of `make generate`)
- `examples/<kind>-slo/kind/` - kind cluster + Helm harness (replaces the prior docker-compose harness)
  - `setup.sh` - creates the cluster and installs the upstream helm chart
  - `teardown.sh` - deletes the cluster
  - `sync.sh` - re-applies the rules + dashboards ConfigMaps after re-running `make generate`
- `deploy/dashboards/` - repo-root OpenSLO Grafana dashboards (`openslo-list.json`, `openslo-detail.json`, `openslo-dashboards.yaml` provider)

### Examples workflow

Each `examples/<kind>-slo/` ships its own `Makefile` with these targets (shells out to `go run .` from the repo root via `git rev-parse --show-toplevel`):

- `make verify` - `go run . load -f <specs> -r` (catch-all sanity check)
- `make generate` - `go run . generate -f <specs> -r -o <rules>`; passes `-r` implicitly
- `make lint-rules` - `promtool check rules` against every generated `<rules>/*.yaml`
- `make clean` - `rm -rf <rules>`
- `oteldemo/` adds: `start-demo` / `stop-demo` (kind cluster lifecycle) and `sync` (`kind/sync.sh`)

Root Makefile targets (`make load FILE=…` / `make validate FILE=…` / `make generate FILE=… OUTPUT=…`) do NOT pass `-r` - use `go run . … -r …` directly when you need it.

## Semconv Codegen Flow

`semconv/registry/` (YAML metrics/attributes) → `semconv/templates/go/` (MiniJinja) → `pkg/semconv/semconv_gen.go`

Run `make semconv-generate` after editing registry YAML or templates. `go generate ./...` runs this before goreleaser builds.

## Key Dependencies

- `github.com/OpenSLO/go-sdk` - official OpenSlo SDK for decoding specs (v0.9.2)
- `github.com/spf13/cobra` - CLI framework
- `log/slog` - structured logging (stdlib)
- `github.com/Masterminds/sprig/v3` - template functions
- OpenTelemetry Weaver - semconv registry management

## CI / Release

- GoReleaser builds linux/darwin binaries, CGO_ENABLED=0
- `before` hooks: `go mod tidy` + `go generate ./...`
- `prerelease: auto` - tags with prerelease markers get prerelease release

## Tooling

- `mise.toml` manages Go (1.26), golangci-lint, weaver, promtool
- `go.mod` declares `go 1.25.5` - auto-upgraded by SDK migration; trust mise for dev
- No `.golangci.yml` - uses defaults

## Testing

Snapshot tests use [`sebdah/goldie/v2`](https://github.com/sebdah/goldie) via the shared helper in `internal/testutil/`:

- `pkg/specstore/specstore_test.go` - spec loading, multi-doc YAML, ref resolution
- `internal/testutil/golden_test.go` - unit tests for the helper itself
- `internal/testutil/golden.go` - `AssertGolden(t, fixtureDir, name, got)`; `name` must include a file extension
- `internal/generator/prometheusgenerator/prometheus_test.go` - table-driven generator suite (`TestGenerate_Golden` covers single-line / multi-line / ratio / multi-dim / tiered cases)
- `internal/generator/prometheusgenerator/labels_test.go` - label rendering helpers

Fixtures live under each package's `testdata/` as `*.golden.yaml`. Update them with `go test ./<pkg>/... -update` after intentional generator/template changes, then visually diff the diff.

Run a single package: `go test ./internal/generator/prometheusgenerator/...`.

## Gotchas

- `generate` rejects: empty `-o`, SLOs without `indicator`
- Both `ThresholdMetric` and `RatioMetric` SLIs supported - ratio SLIs additionally emit `openslo_sli_event_rate_<window>` recording series (see `ratio-slo.yaml` / `ratio-percent-slo.yaml` snapshots)
- Non-OpenSlo YAML files silently skipped (continue on decode error)
- `semconv_gen.go` is auto-generated - never hand-edit
- Feature flags use SLO annotations: `multi-dimensional-sli.openslo.com/dimensions` + `multi-dimensional-sli.openslo.com/label`
- All scripting and testing scratch files (ad-hoc specs, output dirs, fixtures) must live in `./tmp` inside the repo - never `/tmp` or other system-global paths. The `./tmp` dir is gitignored scratch space scoped to this worktree.

### SLI source conventions in `examples/oteldemo/specs/`

- **Path Y** - frontend HTTP SERVER spans. Captures user-originated HTTP calls at the entry point. Filters: `service_name="frontend"` + `span_kind="SPAN_KIND_SERVER"` + `span_name=<route>`. Used by 6 SLOs (ad-availability, cart-availability, product-catalog-availability, recommendation-availability, payment-unreachable, order-processing-latency).
- **Path Y (latency)** - same set + `le=<ms>` histogram bucket, plus `traces_span_metrics_duration_milliseconds_bucket` / `_count`. Used by order-processing-latency (`le="15000"`) and ad-latency (`le="1000"`).
- **Path Y-fauna** - service-side SERVER spans on internal services. Used by image-loading-latency (`service_name="frontend-proxy"`), post-order-email-availability + post-order-email-latency (`service_name="email"`).
- Source-service spans (frontend, image-provider, etc.) are now used rarely because they include flagd client-noise and INTERNAL span pollution. Stick to the path convention above.

### chaos_flag label drift (intentional)

`metadata.labels.chaos_flag` matches the demo user's intuition, not the flagd JSON canonical name. The flagd UI shows different (cleaner) names than the historical spec labels. Both forms exist; align later if we adopt canonical names everywhere.

| SLO spec `chaos_flag` | flagd JSON canonical | match? |
|---|---|---|
| `adServiceFailure` | `adFailure` | ✗ drift |
| `cartServiceFailure` | `cartFailure` | ✗ drift |
| `paymentServiceUnreachable` | `paymentUnreachable` | ✗ drift |
| `recommendationServiceCacheFailure` | `recommendationCacheFailure` | ✗ drift |
| `imageSlowLoad` | `imageSlowLoad` | ✓ |
| `kafkaQueueProblems` | `kafkaQueueProblems` | ✓ |
| `emailMemoryLeak` | `emailMemoryLeak` | ✓ |
| `productCatalogFailure` | `productCatalogFailure` | ✓ |

The metric source for Path Y SLOs lives on `service_name="frontend"` (HTTP SERVER), so the `chaos_flag` label attached to the backend service is **decorative on the recording rule's `chaos_flag` label** - useful for documentation but does NOT drive dashboard filter behavior. Dashboards must filter by `openslo_slo_name=`, not `chaos_flag=`, for Path-Y SLOs.

## SDK API Notes (github.com/OpenSLO/go-sdk)

- `SLIMetricSource.Spec` (not `MetricSourceSpec`) - `map[string]any` containing the query
- `SLOObjective.Target` is `*float64` (pointer), not `float64`
- `SLOTimeWindow.Duration` is `v1.DurationShorthand` (struct), not `string` - use `.String()` for string representation
- `BudgetAdjustment` kind not supported in this SDK version
