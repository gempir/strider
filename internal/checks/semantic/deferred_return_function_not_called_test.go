package semantic

import "testing"

func TestDeferredReturnFunctionNotCalledReportsMissingInvocation(t *testing.T) {
	root := analysisModule(t, `package sample

func setup() func() { return func() {} }

func run() {
	defer setup()
	defer setup()()
}
`)
	registry, err := newRegistry([]string{
		"deferred-return-function-not-called",
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
