package semantic

import "testing"

func TestSingleIterationLoopReportsUnconditionalFirstExit(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func first(values []int) int {
	for _, value := range values {
		if value < 0 {
			use(value)
		}
		return value
	}
	return 0
}

func use(int) {}
`,
	)
	registry, err := newRegistry([]string{
		"single-iteration-loop",
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
