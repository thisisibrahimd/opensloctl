package prometheusgenerator

import (
	"testing"

	v1 "github.com/OpenSLO/go-sdk/pkg/openslo/v1"
	"github.com/stretchr/testify/assert"
)

// TestPromLabelsFromOpenSlo covers the one-value-per-label invariant. The
// validator rejects multi-value Label entries (more than one element) and
// silently drops empty ones; single-string and one-element list forms
// share the same pass-through behavior.
func TestPromLabelsFromOpenSlo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   v1.Labels
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "empty map yields nil",
			input: v1.Labels{},
			want:  map[string]string{},
		},
		{
			name: "single string form (SDK crops to one element)",
			input: v1.Labels{
				"service": []string{"ad"},
			},
			want: map[string]string{"service": "ad"},
		},
		{
			name: "one-element list accepted",
			input: v1.Labels{
				"team": []string{"platform"},
			},
			want: map[string]string{"team": "platform"},
		},
		{
			name: "empty list dropped silently",
			input: v1.Labels{
				"region": []string{},
			},
			want: map[string]string{},
		},
		{
			name: "two-value list rejected",
			input: v1.Labels{
				"region": []string{"us", "eu"},
			},
			wantErr: true,
		},
		{
			name: "mixed valid + invalid rejects the whole SLO",
			input: v1.Labels{
				"team":   []string{"platform"},
				"region": []string{"us", "eu", "ap"},
			},
			wantErr: true,
		},
		{
			name: "hyphenated label name rejected",
			input: v1.Labels{
				"chaos-flag": []string{"kafkaQueueProblems"},
			},
			wantErr: true,
		},
		{
			name: "underscored label name accepted",
			input: v1.Labels{
				"chaos_flag": []string{"kafkaQueueProblems"},
			},
			want: map[string]string{"chaos_flag": "kafkaQueueProblems"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := promLabelsFromOpenSlo(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
