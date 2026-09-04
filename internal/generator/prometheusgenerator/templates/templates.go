package templates

import _ "embed"

//go:embed templates/prometheus-recording-rules.template.yaml
var PrometheusRecordingRuleTemplate string

//go:embed templates/prometheus-alert-rules.template.yaml
var PrometheusAlertRuleTemplate string

type WindowedPrometheusQuery struct {
	Window string `json:"window"`
	Query  string `json:"query"`
}

type AlertCondition struct {
	Expr     string `json:"expr"`
	Severity string `json:"severity"`
}

type AlertGroup struct {
	Conditions  []AlertCondition `json:"conditions"`
	For         string           `json:"for"`
	Thresholds  string           `json:"thresholds"`
	Lookbacks   string           `json:"lookbacks"`
}

type TemplateData struct {
	SloName                   string                     `json:"slo_name"`
	OpensloVersion            string                     `json:"openslo_version"`
	PrometheusQuery           string                     `json:"prometheus_query"`
	WindowedPrometheusQueries []*WindowedPrometheusQuery `json:"windowed_prometheus_queries"`
	Objective                 string                     `json:"objective"`
	IsMulti                   bool                       `json:"is_multi"`
	MultiDimensionalLabel     string                     `json:"multi_dimensional_label"`
	TimeWindowDays            string                     `json:"time_window_days"`
	AlertGroups               map[string]AlertGroup      `json:"alert_groups"`
}

type WindowData struct {
	Window string `json:"window"`
}

var (
	Windows = []string{"5m", "30m", "1h", "3h", "6h", "1d", "3d", "7d", "28d", "30d"}
)
