package prometheusgenerator

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"
	"github.com/thisisibrahimd/opensloctl/internal/feature"
	"github.com/thisisibrahimd/opensloctl/internal/generator"
	"github.com/thisisibrahimd/opensloctl/internal/generator/prometheusgenerator/templates"
	"github.com/thisisibrahimd/opensloctl/pkg/specstore"
)

const (
	RECORDING_RULES_SUFFIX = "-recording-rules.yaml"
	ALERT_RULES_SUFFIX     = "-alert-rules.yaml"
)

var (
	DEFAULT_FILE_MODE = ""
	numberRegex       = regexp.MustCompile("[0-9]+")
	// daysRegex         = regexp.MustCompile("([0-9]+)d")
)

type PrometheusGenerator struct {
	specs *specstore.OpenSLOSpecs
}

func NewPrometheusGenerator(specs *specstore.OpenSLOSpecs) generator.Generator {
	return &PrometheusGenerator{
		specs: specs,
	}
}

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

func (g *PrometheusGenerator) createGeneratedFiles() ([]*generator.GeneratedFile, error) {
	var generatedPrometheusRuleFiles []*generator.GeneratedFile
	// loop through slos
	for _, slo := range g.specs.V1.SLOs {
		slog.Info("generating prometheus recording rule", "slo", slo.Metadata.Name)

		// TODO: support indicator ref
		// Ensure indicator is present
		if slo.Spec.Indicator == nil {
			return nil, fmt.Errorf("indicator is required for slo: %s", slo.Metadata.Name)
		}

		// Pick and generate prom query
		var promQuery string
		if slo.Spec.Indicator.Spec.RatioMetric != nil {
			return nil, fmt.Errorf("ratio metrics are not supported")
		} else {
			metricSource := slo.Spec.Indicator.Spec.ThresholdMetric.MetricSource
			if metricSource.MetricSourceRef != "" {
				slog.Warn("SLI uses metricSourceRef, expected inline Prometheus type", "slo", slo.Metadata.Name, "ref", metricSource.MetricSourceRef)
			} else if metricSource.Type != "Prometheus" {
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

		// template out prom rules
		tpldData := templates.TemplateData{
			SloName:                   slo.Metadata.Name,
			OpensloVersion:            string(slo.APIVersion),
			PrometheusQuery:           windowedPromQueries[0].Query,
			WindowedPrometheusQueries: windowedPromQueries,
			Objective:                 strconv.FormatFloat(*slo.Spec.Objectives[0].Target, 'f', -1, 64),
			IsMulti:                   multiFeatureEnabled,
			MultiDimensionalLabel:     multiDimSliLabel,
			TimeWindowDays:            numberOfDays,
			AlertGroups:               g.buildAlertGroups(slo.Metadata.Name),
		}

		prometheusTemplate := template.Must(template.New("prometheus-recording-rules").Funcs(sprig.FuncMap()).Parse(templates.PrometheusRecordingRuleTemplate))
		var generatedRecordingRules bytes.Buffer
		err := prometheusTemplate.Execute(&generatedRecordingRules, tpldData)
		if err != nil {
			return nil, fmt.Errorf("unable to execute template")
		}

		// create generated file struct
		filename := slo.Metadata.Name + RECORDING_RULES_SUFFIX
		generatedPrometheusRuleFile := &generator.GeneratedFile{
			Path: filename,
			Data: generatedRecordingRules.String(),
		}

		generatedPrometheusRuleFiles = append(generatedPrometheusRuleFiles, generatedPrometheusRuleFile)

		// generate alert rules if alert groups exist
		if len(tpldData.AlertGroups) > 0 {
			alertTemplate := template.Must(template.New("prometheus-alert-rules").Funcs(sprig.FuncMap()).Parse(templates.PrometheusAlertRuleTemplate))
			var generatedAlertRules bytes.Buffer
			err := alertTemplate.Execute(&generatedAlertRules, tpldData)
			if err != nil {
				return nil, fmt.Errorf("unable to execute alert template")
			}

			alertFilename := slo.Metadata.Name + ALERT_RULES_SUFFIX
			generatedAlertRuleFile := &generator.GeneratedFile{
				Path: alertFilename,
				Data: generatedAlertRules.String(),
			}
			generatedPrometheusRuleFiles = append(generatedPrometheusRuleFiles, generatedAlertRuleFile)
		}
	}

	return generatedPrometheusRuleFiles, nil
}

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

	alertGroups := make(map[string]templates.AlertGroup)

	for _, alertPolicyRef := range sloObj.Spec.AlertPolicies {
		polRef := alertPolicyRef.AlertPolicyRef
		policy, ok := g.specs.V1.AlertPolices[polRef]
		if !ok {
			slog.Warn("alert policy not found", "slo", sloName, "policy", polRef)
			continue
		}

		for _, condRef := range policy.Spec.Conditions {
			condKey := condRef.ConditionRef
			condition, ok := g.specs.V1.AlertConditions[condKey]
			if !ok {
				slog.Warn("alert condition not found", "slo", sloName, "condition", condKey)
				continue
			}

			if condition.Spec.Condition.Kind != "burnrate" {
				continue
			}

			severity := condition.Spec.Severity
			threshold := condition.Spec.Condition.Threshold
			lookback := condition.Spec.Condition.LookbackWindow.String()
			alertAfter := condition.Spec.Condition.AlertAfter.String()

			op := "gte"
			if condition.Spec.Condition.Operator != "" {
				op = string(condition.Spec.Condition.Operator)
			}

			burnRateExpr := fmt.Sprintf(
				"openslo_sli_error_rate%s{openslo_slo_name=\"%s\"} / (1 - openslo_slo_objective{openslo_slo_name=\"%s\"}) %s %f",
				lookback, sloName, sloName, op, *threshold,
			)

			group, exists := alertGroups[severity]
			if !exists {
				group = templates.AlertGroup{
					For: alertAfter,
				}
			}

			group.Conditions = append(group.Conditions, templates.AlertCondition{
				Expr: burnRateExpr,
			})

			thresholds := appendIfMissing(group.Thresholds, fmt.Sprintf("%.1fx", *threshold))
			lookbacks := appendIfMissing(group.Lookbacks, lookback)
			group.Thresholds = thresholds
			group.Lookbacks = lookbacks

			alertGroups[severity] = group
		}
	}

	return alertGroups
}

func appendIfMissing(existing, newItem string) string {
	if existing == "" {
		return newItem
	}
	if !strings.Contains(existing, newItem) {
		return existing + " and " + newItem
	}
	return existing
}
