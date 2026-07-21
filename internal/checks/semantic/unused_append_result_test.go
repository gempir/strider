package semantic

import "testing"

func TestUnusedAppendResultReportsUnobservedLocalSlice(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func appendAndForget() {
	values := make([]int, 0, 1)
	values = append(values, 1)
}

func appendAndReturn() []int {
	values := make([]int, 0, 1)
	values = append(values, 1)
	return values
}
`,
	)
	registry, err := newRegistry([]string{
		"unused-append-result",
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
