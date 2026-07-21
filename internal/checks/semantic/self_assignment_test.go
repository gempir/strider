package semantic

import "testing"

func TestSelfAssignmentReportsSideEffectFreeIdentity(t *testing.T) {
	root := analysisModule(t, `package sample

func assign(value int, values []int, next func() int) {
	value = value
	values[next()] = values[next()]
}
`)
	registry, err := newRegistry([]string{
		"self-assignment",
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
