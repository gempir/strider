package semantic

import "testing"

func TestArgumentOverwrittenBeforeUseReportsUnusedIncomingValue(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func replaced(value string) string {
	value = "replacement"
	return value
}

func observed(value string) string {
	use(value)
	value = "replacement"
	return value
}

func use(string) {}
`,
	)
	registry, err := newRegistry([]string{
		"argument-overwritten-before-use",
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
