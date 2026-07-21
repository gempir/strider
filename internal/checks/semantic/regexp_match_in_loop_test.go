package semantic

import "testing"

func TestRegexpMatchInLoopReportsRepeatedCompilation(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "regexp"

func repeated(values []string, dynamic string) {
	for _, value := range values {
		regexp.MatchString("^[a-z]+$", value)
		regexp.MatchString(dynamic, value)
	}
	regexp.MatchString("^[a-z]+$", "outside")
}
`,
	)
	registry, err := newRegistry([]string{
		"regexp-match-in-loop",
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
