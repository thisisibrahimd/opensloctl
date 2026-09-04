package prometheusgenerator

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thisisibrahimd/opensloctl/pkg/specstore"
)

func TestGenerate_QueryOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		file        string
		expectBlock bool
		checkQuery  string
	}{
		{
			name:        "multiline query uses block scalar",
			file:        "multiline-slo.yaml",
			expectBlock: true,
			checkQuery:  "histogram_quantile",
		},
		{
			name:        "single line stays inline",
			file:        "singleline-slo.yaml",
			expectBlock: false,
			checkQuery:  "up",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testdata := filepath.Join("testdata", tt.file)

			specs, err := specstore.GetSpecs([]string{testdata}, false)
			require.NoError(t, err)

			gen := NewPrometheusGenerator(specs).(*PrometheusGenerator)
			files, err := gen.createGeneratedFiles()
			require.NoError(t, err)
			require.Len(t, files, 1)

			content := files[0].Data

			if tt.expectBlock {
				assert.Contains(t, content, "expr: |")
				assert.Contains(t, content, tt.checkQuery)
			} else {
				lines := strings.Split(content, "\n")
				found := false
				for _, line := range lines {
					if strings.Contains(line, "expr:") && strings.Contains(line, tt.checkQuery) {
						assert.NotContains(t, line, "|")
						found = true
					}
				}
				assert.True(t, found, "expected to find expr line for single-line query")
			}
		})
	}
}

func TestTemplate_MultilineDetection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		query       string
		hasNewline  bool
	}{
		{
			name:       "single line no newline",
			query:      `up{job="test"}`,
			hasNewline: false,
		},
		{
			name: "multiline with newline",
			query: `histogram_quantile(0.99,
  sum(rate(bucket[5m])) by (le))`,
			hasNewline: true,
		},
		{
			name: "complex query with division",
			query: `sum(rate(http_requests_total{status=~"2.."}[5m]))
/
sum(rate(http_requests_total[5m]))`,
			hasNewline: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := strings.Contains(tt.query, "\n")
			assert.Equal(t, tt.hasNewline, result)
		})
	}
}
