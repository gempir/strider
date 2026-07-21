package semantic

import "testing"

func TestBenchmarkIterationMutationReportsAssignment(t *testing.T) {
	root := analysisModule(t, `package sample

import "testing"

func BenchmarkWork(b *testing.B) {
	b.N = 1000
}
`)
	registry, err := newRegistry([]string{
		"benchmark-iteration-mutation",
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
