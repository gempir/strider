package semantic

import "testing"

func TestPartiallyTypedConstantGroupReportsDefaultedTypes(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type kind int

const (
	first kind = 1
	second = 2
	third = 3
)

const (
	one kind = 1
	two kind = 2
)

const (
	initial kind = iota
	next
)
`,
	)
	registry, err := newRegistry([]string{
		"partially-typed-constant-group",
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
