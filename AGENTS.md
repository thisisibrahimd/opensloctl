# AGENTS.md

## Commands

```
go build -o opensloctl .          # build binary
go run . load -f <file>           # parse and print OpenSlo specs
go run . generate -f <file> -o <dir>  # generate Prometheus recording rules
golangci-lint run                 # lint (via mise)
go test ./...                     # run tests (none exist yet)
```

## Architecture

- `main.go` → `cmd.Execute()` — single entrypoint
- CLI: cobra-based, two subcommands: `load`, `generate`
- `pkg/specstore/` — loads and sorts OpenSlo YAML files into typed structs
- `internal/generator/prometheusgenerator/` — generates Prometheus recording rule YAML from SLO specs using Go templates (embedded via `//go:embed`)
- `pkg/semconv/` — OpenTelemetry semantic convention constants
- `internal/feature/` — feature flags (multi-dimensional SLI annotations)
- `pkg/util/file.go` — file discovery (recursive YAML/YML finder)

## Key Dependencies

- `github.com/thisisibrahimd/openslo` — OpenSlo SDK for decoding specs
- `github.com/spf13/cobra` — CLI framework
- `github.com/charmbracelet/log` — logging
- `github.com/Masterminds/sprig/v3` — template functions

## CI / Release

- GoReleaser builds linux/darwin binaries, CGO_ENABLED=0
- PR triggers snapshot dry-run; published release triggers real release
- `go mod tidy` + `go generate ./...` run before build

## Tooling

- `mise.toml` manages Go (1.26), golangci-lint, weaver
- No `.golangci.yml` — uses defaults
- No Makefile, Taskfile, or pre-commit hooks

## Gotchas

- `generate` requires `-o` (output directory) — cannot be empty
- `generate` requires `indicator` on SLOs; ratio metrics not supported
- Spec files must be YAML/YML; non-OpenSlo files are silently skipped with a log error
- No tests exist — adding tests requires setting up from scratch
