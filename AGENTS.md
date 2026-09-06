# AGENTS.md

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

- `main.go` → `cmd.Execute()` — single entrypoint
- CLI: cobra-based, three subcommands: `load`, `validate`, `generate`
  - All accept `-f` (filename, repeatable) and `-r` (recursive directory scan)
  - `generate` also requires `-o` (output directory)
  - `validate` runs full validation (load-time + generator-side) without writing files
- `pkg/specstore/loader.go` — loads YAML files via `openslosdk.Decode`, sorts into typed `OpenSloSpecs` struct
- `internal/generator/generator.go` — `Generator` interface
- `internal/generator/prometheusgenerator/` — generates Prometheus rules YAML from SLO specs using Go templates + sprig (embedded via `//go:embed`). One unified output file per SLO: `<slo-name>-rules.yaml` (covering recording rules and, if alert policies are referenced, alert rules via an `openslo-alerts-<slo-name>` group inside the same file).
- `internal/feature/feature.go` — feature flags for multi-dimensional SLI annotations
- `pkg/semconv/semconv_gen.go` — **auto-generated** from semconv registry (do not edit manually)
- `pkg/util/file.go` — recursive YAML/YML file discovery
- `semconv/registry/` — OpenTelemetry Weaver registry YAML (metrics + attributes)
- `semconv/templates/go/` — MiniJinja templates for semconv codegen

## Semconv Codegen Flow

`semconv/registry/` (YAML metrics/attributes) → `semconv/templates/go/` (MiniJinja) → `pkg/semconv/semconv_gen.go`

Run `make semconv-generate` after editing registry YAML or templates. `go generate ./...` runs this before goreleaser builds.

## Key Dependencies

- `github.com/OpenSLO/go-sdk` — official OpenSlo SDK for decoding specs (v0.9.2)
- `github.com/spf13/cobra` — CLI framework
- `log/slog` — structured logging (stdlib)
- `github.com/Masterminds/sprig/v3` — template functions
- OpenTelemetry Weaver — semconv registry management

## CI / Release

- GoReleaser builds linux/darwin binaries, CGO_ENABLED=0
- `before` hooks: `go mod tidy` + `go generate ./...`
- `prerelease: auto` — tags with prerelease markers get prerelease release

## Tooling

- `mise.toml` manages Go (1.26), golangci-lint, weaver
- `go.mod` declares `go 1.25.5` — auto-upgraded by SDK migration; trust mise for dev
- No `.golangci.yml` — uses defaults
- No tests exist — adding tests requires setting up from scratch

## Gotchas

- `generate` rejects: empty `-o`, SLOs without `indicator`, ratio metrics (not supported)
- Only `ThresholdMetric` supported — `RatioMetric` returns error
- Non-OpenSlo YAML files silently skipped (continue on decode error)
- `semconv_gen.go` is auto-generated — never hand-edit
- Feature flags use SLO annotations: `multi-dimensional-sli.openslo.com/dimensions` + `multi-dimensional-sli.openslo.com/label`
- All scripting and testing scratch files (ad-hoc specs, output dirs, fixtures) must live in `./tmp` inside the repo — never `/tmp` or other system-global paths. The `./tmp` dir is gitignored scratch space scoped to this worktree.

## SDK API Notes (github.com/OpenSLO/go-sdk)

- `SLIMetricSource.Spec` (not `MetricSourceSpec`) — `map[string]any` containing the query
- `SLOObjective.Target` is `*float64` (pointer), not `float64`
- `SLOTimeWindow.Duration` is `v1.DurationShorthand` (struct), not `string` — use `.String()` for string representation
- `BudgetAdjustment` kind not supported in this SDK version
