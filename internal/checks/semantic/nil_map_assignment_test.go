package semantic

import "testing"

func TestNilMapAssignmentReportsPanickingWrite(t *testing.T) {
	root := analysisModule(t, `package sample

func write() {
	var values map[string]int
	values["answer"] = 42
}
`)
	registry, err := newRegistry([]string{
		"nil-map-assignment",
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
