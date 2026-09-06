package templates

import _ "embed"

//go:embed templates/prometheus-recording-rules.template.yaml
var PrometheusRulesTemplate string

type WindowedPrometheusQuery struct {
	Window string `json:"window"`
	Query  string `json:"query"`
}

type AlertCondition struct {
	Expr       string `json:"expr"`
	Severity   string `json:"severity"`
	Threshold  string `json:"threshold"`
	Lookback   string `json:"lookback"`
	AlertAfter string `json:"alert_after"`
}

// AlertTier groups conditions that share a tier name. Within a tier the
// conditions are AND-combined; across tiers of the same severity, OR-combined.
// For kinds that do not pair conditions (error-rate, burn-rate,
// multi-burn-rate), each condition becomes its own tier so they're naturally
// OR-combined.
type AlertTier struct {
	Conditions []AlertCondition `json:"conditions"`
}

// AlertGroup carries the per-severity alert data rendered into the
// Prometheus rules template. KindPascal is the PascalCase form of the
// OpenSlo condition kind (e.g. "multi-window-multi-burn-rate" →
// "MultiWindowMultiBurnRate"). KindDescription is the lower-spaced form
// used in alert summary annotations. SloNamePascal is the SLO's kebab-case
// name with hyphens removed and each segment title-cased, so the rendered
// alert name stays a single PascalCase identifier with no underscores.
// NotificationTarget is the resolved name of the AlertPolicy's single
// notification target (e.g. "engineers"), emitted as the
// openslo_notification_target Prometheus label to enable Alertmanager
// routing. Empty when the AlertPolicy doesn't reference a target.
type AlertGroup struct {
	Tiers             []AlertTier `json:"tiers"`
	For               string      `json:"for"`
	Thresholds        string      `json:"thresholds"`
	Lookbacks         string      `json:"lookbacks"`
	ThresholdLabel    string      `json:"threshold_label"`
	KindPascal        string      `json:"kind_pascal"`
	KindDescription   string      `json:"kind_description"`
	SloNamePascal     string      `json:"slo_name_pascal"`
	NotificationTarget string      `json:"notification_target"`
}

type TemplateData struct {
	SloName                   string                     `json:"slo_name"`
	OpensloVersion            string                     `json:"openslo_version"`
	PrometheusQuery           string                     `json:"prometheus_query"`
	WindowedPrometheusQueries []*WindowedPrometheusQuery `json:"windowed_prometheus_queries"`
	// WindowedEventRateQueries mirrors PromQueries but for the event-rate
	// metric (events-per-second). Populated only for RatioMetric SLIs
	// (see HasEventRate); ThresholdMetric SLIs skip event-rate entirely
	// because the spec doesn't expose a parallel event-count query.
	WindowedEventRateQueries []*WindowedPrometheusQuery `json:"windowed_event_rate_queries,omitempty"`
	// HasEventRate gates the openslo_sli_event_rate_* rules in the
	// template. False for ThresholdMetric SLIs.
	HasEventRate bool `json:"has_event_rate"`
	Objective    string `json:"objective"`
	IsMulti      bool   `json:"is_multi"`
	MultiDimensionalLabel     string                     `json:"multi_dimensional_label"`
	TimeWindowDays            string                     `json:"time_window_days"`
	// PeriodWindow is the multi-window key (e.g. "30d") used by the
	// period burn rate meta recording. Empty when no candidate exists
	// (template then emits only current_burn_rate).
	PeriodWindow string `json:"period_window"`
	// ExtraLabels are OpenSlo metadata.labels converted into Prometheus
	// labels (already 1-value validated). The {{ .ExtraLabels }} map can
	// be ranged over to render `key: value` lines.
	ExtraLabels map[string]string `json:"extra_labels"`
	AlertGroups map[string]AlertGroup `json:"alert_groups"`
	// StatusThresholds powers the openslo_slo_status gauge. Always
	// provided — defaults fill in missing annotations independently
	// per specstore.ParseStatusThreshold. The template currently emits
	// the status rule for every SLO; tightening to "only SLOs with
	// alert policies" is a follow-up if non-monitored SLOs become a
	// signal-noise concern.
	StatusThresholds *StatusThresholds `json:"status_thresholds,omitempty"`
}

// StatusThresholds carries the resolved warning/critical/breached
// thresholds for the openslo_slo_status gauge. Floats, not strings,
// so the template can format them straight into PromQL comparisons.
type StatusThresholds struct {
	Warning  float64 `json:"warning"`
	Critical float64 `json:"critical"`
	Breached float64 `json:"breached"`
}

type WindowData struct {
	Window string `json:"window"`
}

var (
	Windows = []string{"5m", "30m", "1h", "3h", "6h", "1d", "3d", "7d", "28d", "30d"}
)
