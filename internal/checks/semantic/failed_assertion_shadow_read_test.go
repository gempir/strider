package semantic

import "testing"

func TestFailedAssertionShadowReadReportsZeroValueRead(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "fmt"

func inspect(value any) {
	if value, ok := value.(int); ok {
		fmt.Println(value)
	} else {
		fmt.Printf("unexpected type %T", value)
	}
	if value, ok := value.(int); ok {
		fmt.Println(value)
	} else {
		value = 0
		fmt.Println(value)
	}
}
`,
	)
	registry, err := newRegistry([]string{
		"failed-assertion-shadow-read",
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
