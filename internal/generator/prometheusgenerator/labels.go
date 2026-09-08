package prometheusgenerator

import (
	"fmt"
	"strings"

	v1 "github.com/OpenSLO/go-sdk/pkg/openslo/v1"
)

// promLabelsFromOpenSlo converts OpenSlo metadata.labels into a flat
// map[string]string of Prometheus labels.
//
// Rules:
//   - Each OpenSlo Label must contain exactly one value. Lists with two or
//     more entries are rejected (multi-value labels are not supported).
//     Empty lists are skipped silently.
//   - The single-string YAML form (`service: ad`) is accepted - the SDK
//     normalizes it to a one-element slice.
//   - Label names must match Prometheus's label name grammar
//     ([a-zA-Z_][a-zA-Z0-9_]*). Hyphens are rejected because they are
//     invalid Prometheus label characters, not silently rewritten.
func promLabelsFromOpenSlo(m v1.Labels) (map[string]string, error) {
	out := make(map[string]string, len(m))
	for k, l := range m {
		if strings.Contains(k, "-") {
			return nil, fmt.Errorf("prom label name %q contains a hyphen; Prometheus label names must match [a-zA-Z_][a-zA-Z0-9_]* (use underscores)", k)
		}
		switch len(l) {
		case 0:
			continue
		case 1:
			out[k] = l[0]
		default:
			return nil, fmt.Errorf("prom label %q has %d values; multi-value labels are not supported (use a single string or a one-element list)", k, len(l))
		}
	}
	return out, nil
}
