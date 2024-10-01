package feature

// this package holds informations about features

// Multi Dimensional SLIs
// This allows multiple slis to spawn out of a single sli definition
// There are two annotaitons needed to support this feature.

const (
	MULTI_DIMENSIONAL_SLI_DIMENSIONS = "multi-dimensional-sli.openslo.com/dimensions" // This tells us the dimensions we want to look into
	MULTI_DIMENSIONAL_SLI_LABEL      = "multi-dimensional-sli.openslo.com/label"      // label annotation tells which prom label has the dimension values
)

var (
	MULTI_DIMENSIONAL_SLI_TEMPLATE = "label_join({{.Query}}, 'openslo_slo_id', '-', 'openslo_slo_id', '{{.Label}}'')"
)

type MultiDimensionalSliTemplateData struct {
	Query string `json:"query"`
	Label string `json:"label"`
}
