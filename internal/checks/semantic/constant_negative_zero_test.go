package semantic

import "testing"

func TestConstantNegativeZeroReportsNormalizedLiterals(t *testing.T) {
	root := analysisModule(t, `package sample

var direct = -0.0
var converted = -float64(0)
var convertedAfter = float32(-0)
`)
	registry, err := newRegistry([]string{
		"constant-negative-zero",
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
