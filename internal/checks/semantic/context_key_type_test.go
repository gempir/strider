package semantic

import "testing"

func TestContextKeyTypeReportsUnsafeKeys(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "context"

type key struct{}
type badKey []byte

func values(ctx context.Context) {
	context.WithValue(ctx, "name", 1)
	context.WithValue(ctx, struct{}{}, 1)
	context.WithValue(ctx, badKey(nil), 1)
	context.WithValue(ctx, key{}, 1)
}
`,
	)
	registry, err := newRegistry([]string{
		"context-key-type",
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
