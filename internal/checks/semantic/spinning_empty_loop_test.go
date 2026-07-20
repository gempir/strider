package semantic

import "testing"

func TestSpinningEmptyLoopReportsUnsafeWait(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func spin(ready *bool) {
	for !*ready {}
}

func dynamic(ready func() bool) {
	for !ready() {}
}

func disabled() {
	for false {}
}
`,
	)
	registry, err := newRegistry([]string{
		"spinning-empty-loop",
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
