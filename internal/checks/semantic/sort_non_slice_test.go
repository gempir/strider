package semantic

import "testing"

func TestSortNonSliceReportsConcreteNonSliceValues(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "sort"

func order(array [3]int, slice []int, dynamic any) {
	less := func(left, right int) bool { return left < right }
	sort.Slice(array, less)
	sort.Slice(nil, less)
	sort.Slice(slice, less)
	sort.Slice(dynamic, less)
}
`,
	)
	registry, err := newRegistry([]string{
		"sort-non-slice",
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
