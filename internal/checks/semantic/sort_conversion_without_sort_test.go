package semantic

import "testing"

func TestSortConversionWithoutSortReportsIneffectiveAssignment(t *testing.T) {
	root := analysisModule(t, `package sample

import "sort"

func order(values []int) {
	values = sort.IntSlice(values)
}
`)
	registry, err := newRegistry([]string{
		"sort-conversion-without-sort",
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
