package prometheusgenerator

import (
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/OpenSLO/go-sdk/pkg/openslo/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thisisibrahimd/opensloctl/internal/generator"
	"github.com/thisisibrahimd/opensloctl/internal/testutil"
	"github.com/thisisibrahimd/opensloctl/pkg/specstore"
)

// ptr returns a pointer to v. Convenience helper for constructing optional
// fields (Target, TargetPercent, etc.) inline in test fixtures.
func ptr[T any](v T) *T { return &v }

// inputEntry pairs an OpenSlo YAML file with the expected generator output
// filename. output == "" means the input is a helper file (e.g. a Service
// shared by another SLO) and produces no file on its own.
type inputEntry struct {
	file   string
	output string
}

// TestGenerate_Golden is the unified snapshot suite for the Prometheus
// generator. Each row declares parallel lists:
//
//   - inputs: the OpenSlo YAML files loaded together (in load order)
//   - outputs: the generator-produced filenames the loader must produce
//     (in any order; matched as a set against the actual output)
//   - golden: one of the output filenames whose bytes get compared to a
//     goldie fixture
//
// Update all fixtures:
//
//	go test ./internal/generator/prometheusgenerator/ -update
func TestGenerate_Golden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputs     []inputEntry
		goldenPick string // exact generated output filename to byte-compare against goldie
	}{
		{
			name: "multiline threshold recording",
			inputs: []inputEntry{
				{file: "multiline-slo.yaml", output: "test-multiline-slo-rules.yaml"},
			},
			goldenPick: "test-multiline-slo-rules.yaml",
		},
		{
			name: "singleline threshold recording",
			inputs: []inputEntry{
				{file: "singleline-slo.yaml", output: "test-singleline-slo-rules.yaml"},
			},
			goldenPick: "test-singleline-slo-rules.yaml",
		},
		{
			name: "ratio target recording",
			inputs: []inputEntry{
				{file: "ratio-slo.yaml", output: "test-ratio-slo-rules.yaml"},
			},
			goldenPick: "test-ratio-slo-rules.yaml",
		},
		{
			name: "ratio targetPercent recording",
			// ratio-slo.yaml supplies the Service referenced by the second input.
			inputs: []inputEntry{
				{file: "ratio-percent-slo.yaml", output: "test-ratio-target-percent-rules.yaml"},
				{file: "ratio-slo.yaml", output: "test-ratio-slo-rules.yaml"},
			},
			goldenPick: "test-ratio-target-percent-rules.yaml",
		},
		{
			name: "tiered burn-rate alerts merged into one file",
			// tiered-slo.yaml supplies the SLO; tiered-alerts.yaml supplies
			// conditions/policies/service/notify that the SLO references.
			// The unified template renders both recording and alert rules
			// into one output file (always named <slo-name>-rules.yaml).
			inputs: []inputEntry{
				{file: "tiered-slo.yaml", output: "test-tiered-slo-rules.yaml"},
				{file: "tiered-alerts.yaml", output: ""},
			},
			goldenPick: "test-tiered-slo-rules.yaml",
		},
		{
			name: "multi-dim SLI emits _unlabeled base + label_join post-process rules",
			// multi-dim-slo.yaml enables the IsMulti template branch via
			// both multi-dimensional-sli.openslo.com annotations on
			// metadata.annotations. Verifies that base recordings get the
			// _unlabeled suffix and that the post-process block emits
			// label_join rules that pivot on MultiDimensionalLabel.
			// multi-dim-service.yaml supplies the Service that the SLO
			// references (the SDK rejects a blank spec.service).
			inputs: []inputEntry{
				{file: "multi-dim-slo.yaml", output: "test-multi-dim-slo-rules.yaml"},
				{file: "multi-dim-service.yaml", output: ""},
			},
			goldenPick: "test-multi-dim-slo-rules.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			paths := make([]string, 0, len(tt.inputs))
			for _, in := range tt.inputs {
				paths = append(paths, filepath.Join("testdata", in.file))
			}

			specs, err := specstore.GetSpecs(paths, false)
			require.NoError(t, err)

			gen := NewPrometheusGenerator(specs).(*PrometheusGenerator)
			files, err := gen.createGeneratedFiles()
			require.NoError(t, err)

			// Verify input → output pairing (declarative, order-sensitive).
			actual := pairedOutputs(t, tt.inputs, files)
			assert.Equal(t, tt.inputs, actual,
				"input/output pairing mismatch: each input should produce its declared output in declared order")

			// goldie compare one declared output's bytes. The fixture filename is
// derived from the actual generated filename: AssertGolden inserts
// ".golden" before the extension (e.g. test-ratio-slo-...-rules.yaml becomes
// test-ratio-slo-...-rules.golden.yaml).
			got := pickByName(t, files, tt.goldenPick)
			testutil.AssertGolden(t, "testdata", tt.goldenPick, []byte(got.Data))
		})
	}
}

// pairedOutputs verifies the generator produced exactly the outputs declared
// in inputs, in input-row order. Inputs with an empty output contribute
// nothing. The function mutates a scratch copy and returns it after matching;
// failing the test produces a clear error naming any missing or extra file.
func pairedOutputs(t *testing.T, inputs []inputEntry, files []*generator.GeneratedFile) []inputEntry {
	t.Helper()

	produced := make([]string, 0, len(files))
	for _, f := range files {
		produced = append(produced, f.Path)
	}

	out := make([]inputEntry, len(inputs))
	copy(out, inputs)

	for i := range out {
		if out[i].output == "" {
			continue
		}
		found := false
		for j := range produced {
			if produced[j] == out[i].output {
				produced = append(produced[:j], produced[j+1:]...)
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected generator output %q for input %q but it was not produced; remaining produced files: %v",
				out[i].output, out[i].file, produced)
		}
	}

	if len(produced) > 0 {
		t.Fatalf("generator produced unexpected additional files: %v", produced)
	}

	return out
}

// pickByName returns the generated file whose Path matches exactly.
func pickByName(t *testing.T, files []*generator.GeneratedFile, name string) *generator.GeneratedFile {
	t.Helper()

	for _, f := range files {
		if f.Path == name {
			return f
		}
	}

	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Path)
	}
	t.Fatalf("no generated file matched name=%q; got: %v", name, names)
	return nil
}

// TestObjectiveFloat covers the Target/TargetPercent/missing branches of
// objectiveFloat. Critical assertions:
//   - TargetPercent = 99.0 → "0.99" (not "0.9900000000000001")
//   - TargetPercent = 99.99 → "0.9999"
//   - Missing both → "0" (defensive default)
func TestObjectiveFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		obj  v1.SLOObjective
		want string
	}{
		{"target 0.99", v1.SLOObjective{Target: ptr(0.99)}, "0.99"},
		{"target 0.9999", v1.SLOObjective{Target: ptr(0.9999)}, "0.9999"},
		{"targetPercent 99", v1.SLOObjective{TargetPercent: ptr(99.0)}, "0.99"},
		{"targetPercent 99.99", v1.SLOObjective{TargetPercent: ptr(99.99)}, "0.9999"},
		{"missing both defaults to 0", v1.SLOObjective{}, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, objectiveFloat(tt.obj))
		})
	}
}

// TestStatusRuleUsesBoolModifier is a regression that locks in the
// Prom 3.x-compatible query shape for the status recording rule.
//
// Background: comparison operators between an instant vector and a
// scalar are filters by default - they preserve the LHS value and
// drop series that don't match. Without the `bool` modifier,
// `(burn_rate >= 14.4) * 3` multiplies the raw burn rate (a float
// like 57.3) instead of returning 0 or 3, so the status gauge
// emits the wrong values and the dashboard's 0/1/2/3 → Healthy/
// Burning/Critical/Breached mapping never matches.
//
// The `bool` modifier has been supported since Prometheus 0.19.0,
// so writing rules works on every Prometheus version in practical
// use. We regression-test the rule every generator produces so a
// future template refactor can't silently regress this.
func TestStatusRuleUsesBoolModifier(t *testing.T) {
	t.Parallel()

// Each row declares a spec fixture set that produces at least
				// one SLO with a generated status recording rule. The
				// threshold defaults (warning=1, critical=6, breached=14.4) come
				// from specstore's ParseStatusThreshold fallback, so fixtures
				// don't need explicit threshold.status.openslo.com/* annotations.
				cases := []struct {
					name  string
					paths []string
				}{
					{
						name:  "multiline SLO",
						paths: []string{"testdata/multiline-slo.yaml"},
					},
					{
						name:  "multi-dim SLO",
						paths: []string{"testdata/multi-dim-service.yaml", "testdata/multi-dim-slo.yaml"},
					},
					{
						name:  "tiered SLO",
						paths: []string{"testdata/tiered-alerts.yaml", "testdata/tiered-slo.yaml"},
					},
				}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			specs, err := specstore.GetSpecs(tc.paths, false)
			require.NoError(t, err)

			gen := NewPrometheusGenerator(specs).(*PrometheusGenerator)
			files, err := gen.createGeneratedFiles()
			require.NoError(t, err)

			require.NotEmpty(t, files, "no generated files produced - spec fixtures probably missing")

			var statusRuleSeen int
			for _, f := range files {
				body := string(f.Data)

				// The status block only appears when StatusThresholds
				// are populated. Skip files without it.
				start := strings.Index(body, "- name: openslo-status-recordings-")
				if start < 0 {
					continue
				}
				rel := strings.Index(body[start:], "\n  - name: openslo-")
				bodyFromStart := body[start:]
				// Find the closest `  - name:` that follows the
				// `- name: openslo-status-recordings-` header. If
				// there's no next group, the status block extends to
				// the end of the body.
				status := bodyFromStart
				if rel >= 0 {
					status = bodyFromStart[:rel]
				}

				// Every comparison inside the status block must use
				// the `bool` modifier - 3 `>= bool` (Warning, Critical,
				// Breached) and 2 `<  bool` (Breached, Critical).
				require.Equalf(t, 3, strings.Count(status, ">= bool"),
					"expected 3 '>= bool' in status block; got %d in %s",
					strings.Count(status, ">= bool"), f.Path)
				require.Equalf(t, 2, strings.Count(status, "<  bool"),
					"expected 2 '<  bool' in status block; got %d in %s",
					strings.Count(status, "<  bool"), f.Path)

				// Highest tier (Breached) must multiply by 3, mid
				// (Critical) by 2, lowest (Warning) by 1 - those are
				// the integer status codes the dashboard maps to text.
				require.Contains(t, status, "* 3")
				require.Contains(t, status, "* 2")
				require.Contains(t, status, "* 1")

				// Sanity guard: no `bool` inside the alert block.
				// Alerts use filter semantics, not coerced 0/1.
				if alertStart := strings.Index(body, "- name: openslo-alerts-"); alertStart > 0 {
					rel := strings.Index(body[alertStart:], "\n  - name: openslo-")
					alertEnd := len(body)
					if rel >= 0 {
						alertEnd = rel + alertStart
					}
					alertBlock := body[alertStart:alertEnd]
					require.NotContainsf(t, alertBlock, ">= bool",
						"alert block must keep filter semantics (no bool modifier); found in %s", f.Path)
					require.NotContainsf(t, alertBlock, "<  bool",
						"alert block must keep filter semantics (no bool modifier); found in %s", f.Path)
				}

				statusRuleSeen++
			}

			require.GreaterOrEqualf(t, statusRuleSeen, 1,
				"no generation produced a status recording rule - specstore fallback thresholds may be broken")
		})
	}
}
