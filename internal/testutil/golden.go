// Package testutil holds test helpers shared across opensloctl packages.
package testutil

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebdah/goldie/v2"
)

// AssertGolden compares got to fixtureDir/<base>.golden.<ext>.
//
// name MUST include a file extension (e.g. "rules.yaml"). Extensions known
// to goldie's default splitter are accepted; the suffix is applied verbatim
// from the extension portion only. Bare names like "rules" are rejected.
//
// Examples:
//
//	AssertGolden(t, "testdata", "rules.yaml", got)
//	// → testdata/rules.golden.yaml
//
//	AssertGolden(t, "testdata", "out.json", got)
//	// → testdata/out.golden.json
//
// Update fixtures: `go test ./... -update`.
func AssertGolden(t *testing.T, fixtureDir, name string, got []byte) {
	t.Helper()

	base, suffix, err := goldenBaseAndSuffix(name)
	if err != nil {
		t.Fatalf("AssertGolden: %v", err)
	}

	g := goldie.New(t,
		goldie.WithFixtureDir(fixtureDir),
		goldie.WithNameSuffix(suffix),
	)
	g.Assert(t, base, got)
}

// goldenBaseAndSuffix splits name into (base, ".golden"+ext). Empty extension
// or empty base is an error.
func goldenBaseAndSuffix(name string) (base, suffix string, err error) {
	ext := filepath.Ext(name)
	if ext == "" {
		return "", "", fmt.Errorf("name %q must include a file extension (e.g. .yaml)", name)
	}
	base = strings.TrimSuffix(name, ext)
	if base == "" {
		return "", "", fmt.Errorf("name %q has empty base before extension %q", name, ext)
	}
	return base, ".golden" + ext, nil
}
