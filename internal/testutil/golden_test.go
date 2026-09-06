package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoldenBaseAndSuffix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantBase  string
		wantSuf   string
		wantError bool
	}{
		{
			name:     "yaml extension",
			input:    "rules.yaml",
			wantBase: "rules",
			wantSuf:  ".golden.yaml",
		},
		{
			name:     "json extension",
			input:    "out.json",
			wantBase: "out",
			wantSuf:  ".golden.json",
		},
		{
			name:     "txt extension",
			input:    "snapshot.txt",
			wantBase: "snapshot",
			wantSuf:  ".golden.txt",
		},
		{
			name:     "deep path",
			input:    "alerts/recording.slo.yaml",
			wantBase: "alerts/recording.slo",
			wantSuf:  ".golden.yaml",
		},
		{
			name:      "no extension rejected",
			input:     "rules",
			wantError: true,
		},
		{
			name:      "empty string rejected",
			input:     "",
			wantError: true,
		},
		{
			name:      "extension only rejected",
			input:     ".yaml",
			wantError: true,
		},
		{
			name:      "dot only rejected",
			input:     ".",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			base, suffix, err := goldenBaseAndSuffix(tt.input)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantBase, base)
			assert.Equal(t, tt.wantSuf, suffix)
		})
	}
}
