package semantic

import "testing"

func TestNilContextReportsNilFirstContextArgument(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "context"

func first(ctx context.Context) {}
func second(value string, ctx context.Context) {}
func generic[T any](ctx context.Context, value T) {}

func check() {
	first(nil)
	generic(nil, 1)
	first(context.TODO())
	second("value", nil)
	_ = (func(context.Context))(nil)
}
`,
	)
	registry, err := newRegistry([]string{
		"nil-context",
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
