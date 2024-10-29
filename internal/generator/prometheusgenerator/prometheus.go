package prometheusgenerator

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/charmbracelet/log"
	"github.com/pkg/errors"
	"github.com/thisisibrahimd/opensloctl/internal/feature"
	"github.com/thisisibrahimd/opensloctl/internal/generator"
	"github.com/thisisibrahimd/opensloctl/internal/generator/prometheusgenerator/templates"
	"github.com/thisisibrahimd/opensloctl/pkg/specstore"
)

const (
	FILENAME_SUFFIX = "-recording-rules.yaml"
)

var (
	DEFAULT_FILE_MODE = ""
	numberRegex       = regexp.MustCompile("[0-9]+")
	// daysRegex         = regexp.MustCompile("([0-9]+)d")
)

type PrometheusGenerator struct {
	specs *specstore.OpenSloSpecs
}

func NewPrometheusGenerator(specs *specstore.OpenSloSpecs) generator.Generator {
	return &PrometheusGenerator{
		specs: specs,
	}

}

func (g *PrometheusGenerator) Generate(outputDirectory string) error {
	if outputDirectory == "" {
		return errors.New("output directory can not be empty")
	}

	log.Info("generating files")
	generatedFiles, err := g.createGeneratedFiles()
	if err != nil {
		return errors.Wrap(err, "unable to create generated files")
	}

	for _, generatedFile := range generatedFiles {
		fullPath := path.Join(outputDirectory, generatedFile.Path)
		log.Info("writing generated files", "file", fullPath)
		err := os.WriteFile(fullPath, generatedFile.Bytes(), 0664)
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
		log.Info("generating prometheus recording rule", "slo", slo.Metadata.Name)

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
				return nil, fmt.Errorf("unable to template the window variable")
			}

			windowedPromQuery := &templates.WindowedPrometheusQuery{
				Window: window,
				Query:  windowedPromQueryBuffer.String(),
			}
			windowedPromQueries = append(windowedPromQueries, windowedPromQuery)
		}

		// extract days in time window
		numberOfDays := numberRegex.FindString(slo.Spec.TimeWindow[0].Duration)

		// template out prom rules
		tpldData := templates.TemplateData{
			SloName:                   slo.Metadata.Name,
			OpensloVersion:            string(slo.APIVersion),
			PrometheusQuery:           windowedPromQueries[0].Query,
			WindowedPrometheusQueries: windowedPromQueries,
			Objective:                 strconv.FormatFloat(slo.Spec.Objectives[0].Target, 'f', -1, 32),
			IsMulti:                   multiFeatureEnabled,
			MultiDimensionalLabel:     multiDimSliLabel,
			TimeWindowDays:            numberOfDays,
		}

		prometheusTemplate := template.Must(template.New("prometheus-recording-rules").Funcs(sprig.FuncMap()).Parse(templates.PrometheusRecordingRuleTemplate))
		var generatedRecordingRules bytes.Buffer
		err := prometheusTemplate.Execute(&generatedRecordingRules, tpldData)
		if err != nil {
			return nil, fmt.Errorf("unable to execute template")
		}

		// create generated file struct
		filename := slo.Metadata.Name + FILENAME_SUFFIX
		generatedPrometheusRuleFile := &generator.GeneratedFile{
			Path: filename,
			Data: generatedRecordingRules.String(),
		}

		generatedPrometheusRuleFiles = append(generatedPrometheusRuleFiles, generatedPrometheusRuleFile)
	}

	return generatedPrometheusRuleFiles, nil
}

// func (g *PrometheusGenerator) generateName() string {

// }

// func (g *PrometheusGenerator) generateFiles() ([]generator.GeneratedFile, error) {
// 	return nil, nil
// }
