package semantic

import "testing"

func TestRegexpFindAllZeroReportsRegexpFindAllWithZeroLimit(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "regexp"

func check(expression *regexp.Regexp, input string, dynamic int) {
	expression.FindAllString(input, 0)
	zero := 0
	expression.FindAllStringIndex(input, zero)
	expression.FindAllString(input, -1)
	expression.FindAllString(input, dynamic)
}
`,
	)
	registry, err := newRegistry([]string{
		"regexp-find-all-zero",
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
