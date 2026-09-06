package prometheusgenerator

import (
	"bytes"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	v1 "github.com/OpenSLO/go-sdk/pkg/openslo/v1"
	"github.com/mdobak/go-xerrors"
	"github.com/pkg/errors"
	"github.com/thisisibrahimd/opensloctl/internal/feature"
	"github.com/thisisibrahimd/opensloctl/internal/generator"
	"github.com/thisisibrahimd/opensloctl/internal/generator/prometheusgenerator/templates"
	"github.com/thisisibrahimd/opensloctl/pkg/specstore"
)

const (
	RULES_SUFFIX = "-rules.yaml"
)

var (
	DEFAULT_FILE_MODE = ""
	numberRegex       = regexp.MustCompile("[0-9]+")
	// daysRegex         = regexp.MustCompile("([0-9]+)d")
)

// PrometheusGenerator renders OpenSlo SLOs as Prometheus recording rules
// (and burn-rate alert rules when AlertPolicies exist). It implements the
// generator.Generator interface.
type PrometheusGenerator struct {
	specs *specstore.OpenSLOSpecs
}

// NewPrometheusGenerator constructs a PrometheusGenerator bound to specs.
// All SLOs in specs must have already passed specstore validation.
func NewPrometheusGenerator(specs *specstore.OpenSLOSpecs) generator.Generator {
	return &PrometheusGenerator{
		specs: specs,
	}
}

// Validate runs every generation-side check (indicator resolution, metric
// source type, label grammar, template execution, alert group structure)
// without writing any files. It reuses createGeneratedFiles — which returns
// in-memory GeneratedFile records — and discards the rendered output. Any
// underlying error is returned to the caller untouched.
func (g *PrometheusGenerator) Validate() error {
	_, err := g.createGeneratedFiles()
	return err
}

// Generate renders each SLO in g.specs to a single Prometheus rules file
// ("<slo-name>-rules.yaml"). When the SLO references satisfied AlertPolicies,
// the file contains both recording rules and alert rules rendered by the
// unified template; otherwise only recording rules are emitted under the
// same filename. Files are written to outputDirectory with 0o664 permissions.
//
// outputDirectory must not be empty. Generation aborts on the first error.
func (g *PrometheusGenerator) Generate(outputDirectory string) error {
	if outputDirectory == "" {
		return errors.New("output directory can not be empty")
	}

	slog.Info("generating files")
	generatedFiles, err := g.createGeneratedFiles()
	if err != nil {
		return errors.Wrap(err, "unable to create generated files")
	}

	for _, generatedFile := range generatedFiles {
		fullPath := path.Join(outputDirectory, generatedFile.Path)
		slog.Info("writing generated files", "file", fullPath)
		err := os.WriteFile(fullPath, generatedFile.Bytes(), 0o664)
		if err != nil {
			return errors.Wrap(err, "unable to write file")
		}
	}

	return nil
}

// createGeneratedFiles renders SLOs to in-memory GeneratedFile records
// without writing to disk. Each SLO produces one rules file named
// "<slo-name>-rules.yaml" — recording rules always; alert rules added when
// at least one AlertPolicy resolves to a satisfied condition.
//
// The SLI source is unpacked per kind:
//   - RatioMetric.counter: good / total PromQL
//   - ThresholdMetric: raw PromQL from the metric source
//
// Query templates substitute {{.Window}} with each multi-window duration
// (5m, 30m, 1h, 3h, 6h, 1d, 3d, 7d, 28d, 30d) at generate time. The
// period burn-rate meta rules use "30d" by default — change PeriodWindow
// in templates.TemplateData to wire it differently.
func (g *PrometheusGenerator) createGeneratedFiles() ([]*generator.GeneratedFile, error) {
	var generatedPrometheusRuleFiles []*generator.GeneratedFile
	// loop through slos
	for _, slo := range g.specs.V1.SLOs {
		slog.Info("generating prometheus recording rule", "slo", slo.Metadata.Name)

		// Resolve the indicator the SLO will be evaluated against.
		// Inline indicator (`spec.indicator`) takes precedence; otherwise fall
		// back to `spec.indicatorRef` and look the SLI up in g.specs. specstore
		// already validated the ref resolves at load time.
		indicator := slo.Spec.Indicator
		if indicator == nil && slo.Spec.IndicatorRef != nil && *slo.Spec.IndicatorRef != "" {
			ref := *slo.Spec.IndicatorRef
			sli, ok := g.specs.V1.SLIs[ref]
			if !ok {
				return nil, fmt.Errorf("SLO %q references SLI %q which is not loaded", slo.Metadata.Name, ref)
			}
			indicator = &v1.SLOIndicatorInline{
				Metadata: sli.Metadata,
				Spec:     sli.Spec,
			}
		}
		if indicator == nil {
			return nil, fmt.Errorf("SLO %q has neither spec.indicator nor spec.indicatorRef set", slo.Metadata.Name)
		}

		// Pick and generate prom query
		var promQuery string
		if indicator.Spec.RatioMetric != nil {
			goodSource := indicator.Spec.RatioMetric.Good.MetricSource
			totalSource := indicator.Spec.RatioMetric.Total.MetricSource
			if goodSource.Type != "Prometheus" || totalSource.Type != "Prometheus" {
				slog.Warn("RatioMetric source is not Prometheus type", "slo", slo.Metadata.Name)
			}
			goodQuery := goodSource.Spec["query"].(string)
			totalQuery := totalSource.Spec["query"].(string)
			promQuery = fmt.Sprintf("(%s)\n/\n(%s)", goodQuery, totalQuery)
		} else {
			metricSource := indicator.Spec.ThresholdMetric.MetricSource
			if metricSource.Type != "Prometheus" {
				slog.Warn("SLI metric source is not Prometheus type", "slo", slo.Metadata.Name, "type", metricSource.Type)
			}
			promQuery = metricSource.Spec["query"].(string)
		}

		// check if features are enabled
		multiDimSliLabel := slo.Metadata.Annotations[feature.MULTI_DIMENSIONAL_SLI_LABEL]
		multiFeatureEnabled := multiDimSliLabel != ""

		// template out the window variable in prom query
		var windowedPromQueries []*templates.WindowedPrometheusQuery
		for _, window := range templates.Windows {
			windowData := &templates.WindowData{Window: window}

			windowPromQueryTemplate := template.Must(template.New("promtheus-query").Parse(promQuery))

			var windowedPromQueryBuffer bytes.Buffer

			err := windowPromQueryTemplate.Execute(&windowedPromQueryBuffer, windowData)
			if err != nil {
				return nil, fmt.Errorf("unable to template the window variable")
			}

			windowedPromQuery := &templates.WindowedPrometheusQuery{
				Window: window,
				Query:  windowedPromQueryBuffer.String(),
			}
			windowedPromQueries = append(windowedPromQueries, windowedPromQuery)
		}

// extract days in time window
	numberOfDays := numberRegex.FindString(slo.Spec.TimeWindow[0].Duration.String())

	// Merge OpenSlo metadata.labels into the Prometheus label set.
	// Core labels (openslo_slo_name, openslo_spec_version, optional
	// openslo_service_name) are stamped by the template; passthrough
	// labels are added here after one-value validation.
	extraLabels, err := promLabelsFromOpenSlo(slo.Metadata.Labels)
	if err != nil {
		return nil, fmt.Errorf("SLO %q: %w", slo.Metadata.Name, err)
	}
	if slo.Spec.Service != "" {
		if _, dup := extraLabels["openslo_service_name"]; !dup {
			extraLabels["openslo_service_name"] = slo.Spec.Service
		}
	}

	// Pick the period window for meta burn rules. Prefer exact match
	// against the multi-window set; fall back to the first window that
	// covers the SLO's full time window. If no candidate exists, leave
	// blank and the template will emit only current-burn-rate.
	periodWindow := ""
	if len(windowedPromQueries) > 0 {
		want := slo.Spec.TimeWindow[0].Duration.String()
		for _, q := range windowedPromQueries {
			if q.Window == want {
				periodWindow = q.Window
				break
			}
		}
		if periodWindow == "" {
			for _, q := range windowedPromQueries {
				if q.Window == "30d" || q.Window == "28d" {
					periodWindow = q.Window
					break
				}
			}
		}
	}

	// template out prom rules
		tpldData := templates.TemplateData{
			SloName:                   slo.Metadata.Name,
			OpensloVersion:            string(slo.APIVersion),
			PrometheusQuery:           windowedPromQueries[0].Query,
			WindowedPrometheusQueries: windowedPromQueries,
			Objective:                 objectiveFloat(slo.Spec.Objectives[0]),
			IsMulti:                   multiFeatureEnabled,
			MultiDimensionalLabel:     multiDimSliLabel,
			TimeWindowDays:            numberOfDays,
			PeriodWindow:              periodWindow,
			ExtraLabels:               extraLabels,
			AlertGroups:               g.buildAlertGroups(slo.Metadata.Name),
		}

		prometheusTemplate := template.Must(template.New("prometheus-rules").Funcs(sprig.FuncMap()).Parse(templates.PrometheusRulesTemplate))
		var generated bytes.Buffer
		if err := prometheusTemplate.Execute(&generated, tpldData); err != nil {
			return nil, fmt.Errorf("unable to execute template: %w", err)
		}

		// File name is always <slo-name>-rules.yaml. The unified template
		// produces only recording rules when AlertGroups is empty, and
		// adds an openslo-alerts-<slo-name> group when alerts are present.
		filename := slo.Metadata.Name + RULES_SUFFIX

		generatedPrometheusRuleFile := &generator.GeneratedFile{
			Path: filename,
			Data: generated.String(),
		}

		generatedPrometheusRuleFiles = append(generatedPrometheusRuleFiles, generatedPrometheusRuleFile)
	}

	return generatedPrometheusRuleFiles, nil
}

// buildAlertGroups resolves an SLO's AlertPolicy references into a map
// of severity → AlertGroup, dispatching on each condition's kind to emit
// the correct Prometheus alert expression and tier structure.
//
// Kind matrix (see pkg/specstore for full kind semantics):
//
//   - error-rate:                raw SLI error rate vs absolute threshold.
//     1 condition per severity; tier = condition name.
//   - burn-rate:                 single burn rate multiplier; 1 condition per
//     severity; tier = condition name.
//   - multi-burn-rate:           multiple burn-rate conditions OR-ed per
//     severity; each condition is its own tier.
//   - multi-window-multi-burn-rate: short+long window pairs AND-ed within a
//     tier (derived from the condition name by stripping the trailing
//     "-<lookbackWindow>" suffix) and OR-ed across tiers. Legacy default
//     that retains the prior single-kind="burnrate" tiering behavior.
//
// Unknown policy/condition refs are logged and skipped. Conditions not in
// the four supported kinds are skipped (ValidateRefs already rejects them
// at load time, so reaching here implies a misconfiguration that should be
// surfaced loudly via slog.Error rather than as a panic).
//
// Each group's For field comes from the shortest AlertAfter across its
// conditions; thresholds and lookbacks are joined " and "-separated for
// display in annotations.
func (g *PrometheusGenerator) buildAlertGroups(sloName string) map[string]templates.AlertGroup {
	if g.specs == nil {
		return nil
	}

	sloObj, ok := g.specs.V1.SLOs[sloName]
	if !ok {
		return nil
	}
	if len(sloObj.Spec.AlertPolicies) == 0 {
		return nil
	}

	// Track per-severity: ordered tier names → slice of conditions, plus aggregates.
	type severityState struct {
		kind              specstore.AlertConditionKind
		tierOrder         []string
		tierIndex         map[string]int
		tiers             []templates.AlertTier
		thresholds        string
		lookbacks         string
		for_              string // shortest alertAfter across all conditions in this severity
		notificationTarget string // resolved spec.target of the first AlertPolicy
	}
	stateBySev := make(map[string]*severityState)

	for _, alertPolicyRef := range sloObj.Spec.AlertPolicies {
		polRef := alertPolicyRef.AlertPolicyRef
		policy, ok := g.specs.V1.AlertPolices[polRef]
		if !ok {
			slog.Warn("alert policy not found", "slo", sloName, "policy", polRef)
			continue
		}

		// Resolve the single notification target this AlertPolicy carries.
		// ValidateRefs already enforced len(notificationTargets) <= 1, and
		// resolved every targetRef at load time. We collect the resolved
		// spec.target string here so the alert rule can carry an
		// openslo_notification_target label for Alertmanager routing.
		var policyTarget string
		if len(policy.Spec.NotificationTargets) == 1 {
			ntRef := policy.Spec.NotificationTargets[0].TargetRef
			if nt, ok := g.specs.V1.AlertNotificationTargets[ntRef]; ok {
				policyTarget = nt.Spec.Target
			}
		}

		for _, condRef := range policy.Spec.Conditions {
			condKey := condRef.ConditionRef
			condition, ok := g.specs.V1.AlertConditions[condKey]
			if !ok {
				slog.Warn("alert condition not found", "slo", sloName, "condition", condKey)
				continue
			}

			kind := specstore.AlertConditionKind(condition.Spec.Condition.Kind)

			// Map legacy "burnrate" to the extended multi-window-multi-burn-rate
			// kind so older YAML keeps working without modification.
			if kind == "burnrate" {
				kind = specstore.KindMultiWindowMultiBurnRate
				slog.Warn("legacy AlertCondition kind 'burnrate' detected; rename to 'multi-window-multi-burn-rate' to silence this message", "slo", sloName, "condition", condKey)
			}

			if !specstore.ValidKind(kind) {
				slog.Error("unsupported AlertCondition kind; skipping", "slo", sloName, "condition", condKey, "kind", condition.Spec.Condition.Kind)
				continue
			}

			// Mixed-kind checks per severity come after the loop, once we know
			// the full set of conditions per severity. Per-condition error-rate
			// threshold range is enforced by ValidateRefs; here we surface
			// multi-type structural problems at the SLO level.
			if condition.Spec.Condition.Threshold == nil {
				slog.Error("AlertCondition missing threshold; skipping", "slo", sloName, "condition", condKey)
				continue
			}
			thresholdVal := *condition.Spec.Condition.Threshold

			severity := condition.Spec.Severity
			lookback := condition.Spec.Condition.LookbackWindow.String()
			alertAfter := ""
			if condition.Spec.Condition.AlertAfter != nil {
				alertAfter = condition.Spec.Condition.AlertAfter.String()
			}
			op := promQLOperator(condition.Spec.Condition.Operator)

			expr := buildAlertExpr(kind, thresholdVal, lookback, sloName, string(op))

			// Tier derivation depends on kind. For kinds that pair conditions
			// (multi-window-multi-burn-rate), the tier is the condition name
			// with the "-<lookback>" suffix stripped. For all other kinds,
			// every condition is its own tier (each condition is naturally
			// OR-ed across tiers).
			suffix := "-" + lookback
			tierName := condition.Metadata.Name
			if kind == specstore.KindMultiWindowMultiBurnRate {
				if !strings.HasSuffix(condition.Metadata.Name, suffix) {
					slog.Error("multi-window-multi-burn-rate condition name must end with -<lookbackWindow> suffix; skipping", "slo", sloName, "condition", condKey, "lookback", lookback)
					continue
				}
				tierName = strings.TrimSuffix(condition.Metadata.Name, suffix)
			}

			thresholdDisplay, _ := formatThreshold(kind, thresholdVal, op)

			cond := templates.AlertCondition{
				Expr:       expr,
				Severity:   severity,
				Threshold:  thresholdDisplay,
				Lookback:   lookback,
				AlertAfter: alertAfter,
			}

			state, ok := stateBySev[severity]
			if !ok {
				state = &severityState{
					kind:     kind,
					tierIndex: map[string]int{},
				}
				stateBySev[severity] = state
			}

			if state.kind != kind {
				slog.Error("mixed AlertCondition kinds within the same severity; skipping offending condition", "slo", sloName, "severity", severity, "expected", state.kind, "got", kind, "condition", condKey)
				continue
			}

			if idx, exists := state.tierIndex[tierName]; exists {
				state.tiers[idx].Conditions = append(state.tiers[idx].Conditions, cond)
			} else {
				state.tierIndex[tierName] = len(state.tiers)
				state.tierOrder = append(state.tierOrder, tierName)
				state.tiers = append(state.tiers, templates.AlertTier{Conditions: []templates.AlertCondition{cond}})
			}

			state.thresholds = appendIfMissing(state.thresholds, thresholdDisplay)
			state.lookbacks = appendIfMissing(state.lookbacks, lookback)
			if state.for_ == "" || alertAfter < state.for_ {
				state.for_ = alertAfter
			}

			// Mismatched notification targets between policies of the same
			// severity can't be represented in a single openslo_notification_target
			// label; surface loudly and leave the label empty so the issue is
			// obvious in the generated rules rather than silently picking one.
			if policyTarget != "" {
				if state.notificationTarget == "" {
					state.notificationTarget = policyTarget
				} else if state.notificationTarget != policyTarget {
					slog.Error("AlertPolicy notification targets differ within severity; openslo_notification_target label omitted", "slo", sloName, "severity", severity, "have", state.notificationTarget, "got", policyTarget, "policy", polRef)
					state.notificationTarget = ""
				}
			}
		}
	}

	// Structural validation per severity, applying all rules uniformly.
	for severity, state := range stateBySev {
		if err := validateSeverityStructure(severity, state.kind, state.tierOrder, state.tiers); err != nil {
			slog.Error("alert condition structure invalid; alerts may not fire correctly", "slo", sloName, "severity", severity, "error", err)
		}
	}

	alertGroups := make(map[string]templates.AlertGroup, len(stateBySev))
	for severity, state := range stateBySev {
		alertGroups[severity] = templates.AlertGroup{
			Tiers:              state.tiers,
			For:                state.for_,
			Thresholds:         state.thresholds,
			Lookbacks:          state.lookbacks,
			KindPascal:         specstore.KindPascal(state.kind),
			KindDescription:    specstore.KindDescription(state.kind),
			SloNamePascal:      specstore.SloNamePascal(sloName),
			NotificationTarget: state.notificationTarget,
		}
	}
	return alertGroups
}

// buildAlertExpr returns the PromQL expression for a single AlertCondition.
// All expressions include {openslo_slo_name="<slo>"} so the alert is anchored
// to one SLO. error-rate emits a raw SLI error-rate comparison; burn-rate
// families normalize by (1 - error_budget) to produce a multiplier.
func buildAlertExpr(kind specstore.AlertConditionKind, threshold float64, lookback, sloName, op string) string {
	switch kind {
	case specstore.KindErrorRate:
		return fmt.Sprintf(
			`openslo_sli_error_rate_%s{openslo_slo_name="%s"} %s %f`,
			lookback, sloName, op, threshold,
		)
	case specstore.KindBurnRate, specstore.KindMultiBurnRate, specstore.KindMultiWindowMultiBurnRate:
		return fmt.Sprintf(
			`openslo_sli_error_rate_%s{openslo_slo_name="%s"} / (1 - openslo_slo_objective{openslo_slo_name="%s"}) %s %f`,
			lookback, sloName, sloName, op, threshold,
		)
	default:
		// Defensive default: emit a non-firing predicate. Specstore validation
		// should have rejected an unknown kind upstream.
		return `vector(0)`
	}
}

// formatThreshold renders a threshold value for human-readable display in
// alert annotations. error-rate uses the raw float ("0.001"); burn-rate
// families append "x" to signal the multiplier ("14.4x"). The second
// return value is retained for symmetry with earlier designs and to make
// it easy to add an op-prefixed label later without changing call sites.
func formatThreshold(kind specstore.AlertConditionKind, threshold float64, _ string) (string, string) {
	if kind == specstore.KindErrorRate {
		s := strconv.FormatFloat(threshold, 'f', -1, 64)
		return s, s
	}
	s := fmt.Sprintf("%.1fx", threshold)
	return s, s
}

// validateSeverityStructure enforces per-kind cross-condition structural
// requirements. Errors are returned for the generator to slog.Error; load-time
// validation already happened in specstore, so this is defense in depth.
//
//   - error-rate, burn-rate:       exactly 1 condition per severity.
//   - multi-burn-rate:             >=2 conditions per severity.
//   - multi-window-multi-burn-rate: >=2 tiers, each tier >=2 conditions,
//     and all conditions within a tier share the same threshold.
func validateSeverityStructure(severity string, kind specstore.AlertConditionKind, tierOrder []string, tiers []templates.AlertTier) error {
	total := 0
	for _, t := range tiers {
		total += len(t.Conditions)
	}

	switch kind {
	case specstore.KindErrorRate, specstore.KindBurnRate:
		if total != 1 {
			return xerrors.Newf("kind %q severity %q expects exactly 1 condition, got %d", kind, severity, total)
		}
	case specstore.KindMultiBurnRate:
		if total < 2 {
			return xerrors.Newf("kind %q severity %q expects >=2 conditions, got %d; use 'burn-rate' for a single condition", kind, severity, total)
		}
	case specstore.KindMultiWindowMultiBurnRate:
		if len(tiers) < 2 {
			return xerrors.Newf("kind %q severity %q expects >=2 tiers, got %d", kind, severity, len(tiers))
		}
		for _, t := range tiers {
			if len(t.Conditions) < 2 {
				return xerrors.Newf("kind %q severity %q has a tier with only %d condition (need >=2 for short/long pair)", kind, severity, len(t.Conditions))
			}
			first := thresholdValue(t.Conditions[0].Threshold)
			for _, c := range t.Conditions {
				if thresholdValue(c.Threshold) != first {
					return xerrors.Newf("kind %q severity %q tier has mismatched thresholds %q and %q (all conditions in a tier must share the same burn multiplier)", kind, severity, t.Conditions[0].Threshold, c.Threshold)
				}
			}
		}
	}
	return nil
}

// thresholdValue parses a human-readable threshold back to a float for
// comparison purposes. error-rate thresholds render without suffix ("0.001");
// burn-rate family thresholds render with an "x" suffix ("14.4x"). Both
// forms are parseable as floats.
func thresholdValue(s string) float64 {
	s = strings.TrimSuffix(s, "x")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// promQLOperator maps an OpenSlo SDK v1.Operator ("gt","gte","lt","lte")
// to the equivalent PromQL binary operator. Default for empty or unknown
// input is ">=" (the canonical "burn-rate at or above" comparison).
func promQLOperator(op v1.Operator) string {
	switch op {
	case v1.OperatorGT:
		return ">"
	case v1.OperatorGTE, "":
		return ">="
	case v1.OperatorLT:
		return "<"
	case v1.OperatorLTE:
		return "<="
	default:
		return string(op)
	}
}

// objectiveFloat formats an SLOObjective's success target for embedding
// in recording-rule exprs. Resolution order:
//   - Target (0.0-1.0 scale): emitted verbatim with strconv %g
//   - TargetPercent (0.0-100.0 scale): divided by 100, rounded to 4 decimals
//     to suppress IEEE-754 dust (e.g. 99.9 → "0.999")
//   - Neither: "0" placeholder
func objectiveFloat(obj v1.SLOObjective) string {
	if obj.Target != nil {
		return strconv.FormatFloat(*obj.Target, 'f', -1, 64)
	}
	if obj.TargetPercent != nil {
		return strconv.FormatFloat(math.Round(*obj.TargetPercent*100)/10000, 'f', -1, 64)
	}
	return "0"
}

// appendIfMissing returns "a and b" when joining items for display. Empty
// existing gets replaced; non-empty gets " and "-separated only if the new
// item isn't already present. Used to merge burn-rate thresholds and
// lookback windows into single readable strings for alert annotations.
func appendIfMissing(existing, newItem string) string {
	if existing == "" {
		return newItem
	}
	if !strings.Contains(existing, newItem) {
		return existing + " and " + newItem
	}
	return existing
}
