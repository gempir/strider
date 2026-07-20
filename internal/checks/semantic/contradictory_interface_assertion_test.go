package semantic

import "testing"

func TestContradictoryInterfaceAssertionReportsConflictingMethod(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type source interface { Read() int }
type impossible interface { Read() string }
type compatible interface { Write() string }

func inspect(value source) {
	_, _ = value.(impossible)
	_, _ = value.(compatible)
}

`,
	)
	registry, err := newRegistry([]string{
		"contradictory-interface-assertion",
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
