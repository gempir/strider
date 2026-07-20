package semantic

import "testing"

func TestDynamicPrintfReportsLoneDynamicPrintfFormats(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"fmt"
	"os"
)

func check(message string) {
	fmt.Printf(message)
	fmt.Fprintf(os.Stdout, message)
	fmt.Printf("%s", message)
	fmt.Fprintf(os.Stdout, "%s", message)
}
`,
	)
	registry, err := newRegistry([]string{
		"dynamic-printf",
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
