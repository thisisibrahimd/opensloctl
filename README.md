# opensloctl

Generate Prometheus recording rules from OpenSlo specs.

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

## Semantic Conventions

opensloctl defines a registry of metrics and attributes for SLO telemetry. The registry lives in `semconv/registry/` and is used to generate `pkg/semconv/semconv_gen.go`.

### Attributes

| Attribute | Type | Description |
|---|---|---|
| `openslo.slo.name` | string | The name of the SLO as defined in the OpenSlo spec. |
| `openslo.spec.version` | string | The OpenSlo API version of the SLO spec. |

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
