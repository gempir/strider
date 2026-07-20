package semantic

import "testing"

func TestDuplicateTrimCutsetReportsRepeatedRunes(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "strings"

const prefix = "letter"

func trim(value, dynamic string) {
	strings.TrimLeft(value, prefix)
	strings.Trim(value, "abc")
	strings.TrimRight(value, dynamic)
}
`,
	)
	registry, err := newRegistry([]string{
		"duplicate-trim-cutset",
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
