package semantic

import "testing"

func TestDeferCloseBeforeErrorCheckReportsEarlyDefer(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type resource struct{}
func (*resource) Close() error { return nil }
func open() (*resource, error) { return nil, nil }

func use() error {
	resource, err := open()
	defer resource.Close()
	if err != nil {
		return err
	}
	return nil
}
`,
	)
	registry, err := newRegistry([]string{
		"defer-close-before-error-check",
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
