package semantic

import "testing"

func TestRandomBoundOneReportsConstantZeroRange(t *testing.T) {
	root := analysisModule(t, `package sample

import "math/rand"

func choice() int {
	return rand.Intn(1)
}
`)
	registry, err := newRegistry([]string{
		"random-bound-one",
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
