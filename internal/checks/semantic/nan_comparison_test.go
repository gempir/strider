package semantic

import "testing"

func TestNaNComparisonReportsDirectComparison(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "math"

func isMissing(value float64) bool {
	return value == math.NaN()
}

func isMissingCorrectly(value float64) bool {
	return math.IsNaN(value)
}
`,
	)
	registry, err := newRegistry([]string{
		"nan-comparison",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnosticGolden(t, diagnostics)
}
