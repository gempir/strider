package semantic

import "testing"

func TestPossibleNilDereferenceReportsUnprotectedPath(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func unsafe(value *int, report func()) int {
	if value == nil {
		report()
	}
	return *value
}

func guarded(value *int) int {
	if value != nil {
		return *value
	}
	return 0
}

func terminating(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
`,
	)
	registry, err := newRegistry([]string{
		"possible-nil-dereference",
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
