package semantic

import "testing"

func TestUnreachableTypeSwitchCaseReportsSubsumedInterface(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type reader interface { Read([]byte) (int, error) }
type readCloser interface {
	reader
	Close() error
}

func classify(value any) {
	switch value.(type) {
	case reader:
	case readCloser:
	}
}
`,
	)
	registry, err := newRegistry([]string{
		"unreachable-type-switch-case",
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
