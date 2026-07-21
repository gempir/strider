package semantic

import "testing"

func TestImpossibleIntegerComparisonReportsTypeRangeFacts(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func compare(value uint8) bool {
	belowZero := value < 0
	atLeastZero := value >= 0
	aboveMaximum := value > 255
	return belowZero || atLeastZero || aboveMaximum
}
`,
	)
	registry, err := newRegistry([]string{
		"impossible-integer-comparison",
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
