package semantic

import "testing"

func TestUnchangedLoopConditionReportsWrongCounter(t *testing.T) {
	root := analysisModule(t, `package sample

func loop(limit int) {
	other := 0
	for index := 0; index < limit; other++ {
		if other > limit {
			return
		}
	}
}
`)
	registry, err := newRegistry([]string{
		"unchanged-loop-condition",
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
