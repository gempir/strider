package semantic

import "testing"

func TestContextStoredInStructReportsContextField(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "context"

type stored struct {
	ctx context.Context
}

type explicit struct {
	name string
}

func (explicit) Run(ctx context.Context) { _ = ctx }
`,
	)
	registry, err := newRegistry([]string{
		"context-stored-in-struct",
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
