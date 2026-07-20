package semantic

import "testing"

func TestIneffectiveValueReceiverAssignmentReportsUnreadFieldWrite(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type item struct {
	name string
}

func (value item) rename(name string) {
	value.name = name
}

func (value item) renamed(name string) string {
	value.name = name
	return value.name
}

func (value *item) renameInPlace(name string) {
	value.name = name
}
`,
	)
	registry, err := newRegistry([]string{
		"ineffective-value-receiver-assignment",
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
