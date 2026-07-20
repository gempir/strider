package semantic

import "testing"

func TestInvalidStrconvArgumentReportsInvalidConstants(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "strconv"

func parse(value string, dynamic int) {
	strconv.ParseInt(value, 1, 128)
	strconv.ParseFloat(value, 16)
	strconv.FormatFloat(1, 'q', -1, 64)
	strconv.ParseInt(value, 10, 64)
	strconv.ParseInt(value, dynamic, 64)
}
`,
	)
	registry, err := newRegistry([]string{
		"invalid-strconv-argument",
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
