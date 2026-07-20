package semantic

import "testing"

func TestOverwrittenBeforeUseReportsDeadAssignedValue(t *testing.T) {
	root := analysisModule(t, `package sample

func calculate() int { return 1 }

func result() int {
	value := calculate()
	value = calculate()
	return value
}
`)
	registry, err := newRegistry([]string{
		"overwritten-before-use",
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
