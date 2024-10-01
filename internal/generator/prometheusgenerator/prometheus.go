package prometheusgenerator

import (
	"bytes"
	"strconv"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/charmbracelet/log"
	"github.com/thisisibrahimd/opensloctl/internal/feature"
	"github.com/thisisibrahimd/opensloctl/internal/generator"
	"github.com/thisisibrahimd/opensloctl/internal/generator/prometheusgenerator/templates"
	"github.com/thisisibrahimd/opensloctl/pkg/spec_store"
)

type PrometheusGenerator struct {
	specStore *spec_store.SpecStore
}

func NewPrometheusGenerator(specStore *spec_store.SpecStore) generator.Generator {
	return &PrometheusGenerator{
		specStore: specStore,
	}

}

func (g *PrometheusGenerator) Generate(outputDirectory string) error {
	// loop through slos

	for _, slo := range g.specStore.Store.V1.SLOs {
		log.Info("generating prometheus recording rule", "slo", slo.Metadata.Name)

		// TODO: support indicator ref
		// Ensure indicator is present
		if slo.Spec.Indicator == nil {
			log.Fatal("indicator is required. indicator ref is not supported")
		}

		// Pick and generate prom query
		var promQuery string
		if slo.Spec.Indicator.Spec.RatioMetric != nil {
			log.Fatal("ratio metrics are not supported yet")
		} else {
			promQuery = slo.Spec.Indicator.Spec.ThresholdMetric.MetricSource.MetricSourceSpec["query"].(string)
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
				log.Fatal("failed to template window", "err", err)
			}

			windowedPromQuery := &templates.WindowedPrometheusQuery{
				Window: window,
				Query:  windowedPromQueryBuffer.String(),
			}
			windowedPromQueries = append(windowedPromQueries, windowedPromQuery)
		}

		// template out prom rules
		tpldData := templates.TemplateData{
			SloName:                   slo.Metadata.Name,
			OpensloVersion:            string(slo.APIVersion),
			PrometheusQuery:           windowedPromQueries[0].Query,
			WindowedPrometheusQueries: windowedPromQueries,
			Objective:                 strconv.FormatFloat(slo.Spec.Objectives[0].Target, 'f', -1, 32),
			IsMulti:                   multiFeatureEnabled,
			MultiDimensionalLabel:     multiDimSliLabel,
			TimeWindowDays:            "30d",
		}

		prometheusTemplate := template.Must(template.New("prometheus-recording-rules").Funcs(sprig.FuncMap()).Parse(templates.PrometheusRecordingRuleTemplate))
		var generatedRecordingRules bytes.Buffer
		err := prometheusTemplate.Execute(&generatedRecordingRules, tpldData)
		if err != nil {
			log.Error("could not execute template", "err", err)
			return err
		}

		log.Info(generatedRecordingRules.String())

	}

	return nil
}

// func (g *PrometheusGenerator) generateFiles() ([]generator.GeneratedFile, error) {
// 	return nil, nil
// }
