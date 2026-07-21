package semantic

import "testing"

func TestOddPairedArgumentsReportsKnownOddLengths(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "strings"

func pairs(values ...string) {
	if len(values)%2 != 0 {
		panic("pairs required")
	}
}

func calls() {
	strings.NewReplacer("a", "b", "orphan")
	pairs("a", "b", "orphan")
	strings.NewReplacer("a", "b")
	pairs("a", "b")
}
`,
	)
	registry, err := newRegistry([]string{
		"odd-paired-arguments",
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
