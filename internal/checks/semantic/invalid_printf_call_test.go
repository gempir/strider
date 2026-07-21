package semantic

import "testing"

func TestInvalidPrintfCallReportsFormatAndArgumentMismatches(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "fmt"

func invalid() {
	fmt.Printf("%d", "wrong")
	fmt.Printf("%d")
	fmt.Printf("%*s", "wide", "value")
	fmt.Printf("plain", 1)
	fmt.Printf("%z", 1)
}

func clean() {
	fmt.Printf("%d %s", 1, "value")
	fmt.Printf("%[2]d", "unused", 2)
	fmt.Printf("%*s", 4, "value")
}
`,
	)
	registry, err := newRegistry([]string{
		"invalid-printf-call",
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
