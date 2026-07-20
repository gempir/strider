package semantic

import "testing"

func TestNegativeLengthCapacityComparisonReportsImpossibleCheck(t *testing.T) {
	root := analysisModule(t, `package sample

func impossible(values []int) bool {
	return len(values) < 0 || 0 > cap(values)
}
`)
	registry, err := newRegistry([]string{
		"negative-length-capacity-comparison",
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
