package semantic

import "testing"

func TestCaseInsensitiveStringComparisonReportsAllocatingComparison(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "strings"

func equal(left, right string) bool {
	return strings.ToLower(left) == strings.ToLower(right)
}

func differentConversions(left, right string) bool {
	return strings.ToLower(left) == strings.ToUpper(right)
}

func efficient(left, right string) bool {
	return strings.EqualFold(left, right)
}
`,
	)
	registry, err := newRegistry([]string{
		"case-insensitive-string-comparison",
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
