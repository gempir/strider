package semantic

import "testing"

func TestNeverNilComparisonReportsMadeSliceCheck(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func impossible() bool {
	values := make([]int, 0)
	if values == nil {
		return true
	}
	return false
}

func possible() bool {
	var values []int
	return values == nil
}
`,
	)
	registry, err := newRegistry([]string{
		"never-nil-comparison",
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
