// Package feature holds cross-cutting flags and small inputs that shape
// opensloctl generator behavior.
package feature

const (
	// MULTI_DIMENSIONAL_SLI_DIMENSIONS — annotation listing the OpenSlo
	// SLO dimension values to expand into separate Prometheus recording
	// series.
	MULTI_DIMENSIONAL_SLI_DIMENSIONS = "multi-dimensional-sli.openslo.com/dimensions"

	// MULTI_DIMENSIONAL_SLI_LABEL — annotation naming the Prometheus label
	// that carries each dimension value (joined into openslo_slo_id later).
	MULTI_DIMENSIONAL_SLI_LABEL = "multi-dimensional-sli.openslo.com/label"
)

var (
	MULTI_DIMENSIONAL_SLI_TEMPLATE = "label_join({{.Query}}, 'openslo_slo_id', '-', 'openslo_slo_id', '{{.Label}}'')"
)

type MultiDimensionalSliTemplateData struct {
	Query string `json:"query"`
	Label string `json:"label"`
}
