package semantic

import "testing"

func TestInterfaceMethodLimitConservativeCases(t *testing.T) {
	assertCheckDiagnostics(
		t,
		"interface-method-limit",
		`package sample

type focused interface {
	A(); B(); C(); D(); E(); F(); G(); H(); I(); J()
}

type bloated interface {
	A(); B(); C(); D(); E(); F(); G(); H(); I(); J(); K()
}
`,
		1,
		"11 methods",
	)
}
