package semantic

import "testing"

func TestPointlessIntegerMathReportsConvertedInteger(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "math"

func rounded(count int) float64 {
	return math.Ceil(float64(count))
}

func roundedMeasurement(value float64) float64 {
	return math.Ceil(value)
}
`,
	)
	registry, err := newRegistry([]string{
		"pointless-integer-math",
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
