package semantic

import "testing"

func TestImpossibleInterfaceNilComparisonReportsTypedNil(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type problem struct{}
func (*problem) Error() string { return "problem" }

func typedNil() error {
	var value *problem
	return value
}

func impossible() bool {
	return typedNil() == nil
}
`,
	)
	registry, err := newRegistry([]string{
		"impossible-interface-nil-comparison",
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
