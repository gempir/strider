package semantic

import "testing"

func TestInvalidUTF8ReportsInvalidUTF8StringArguments(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "strings"

func check(dynamic string) {
	strings.Trim("value", "\xff")
	invalid := "\x80"
	strings.ContainsAny("value", invalid)
	strings.Trim("value", "é")
	strings.Trim("value", dynamic)
}
`,
	)
	registry, err := newRegistry([]string{
		"invalid-utf8",
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
