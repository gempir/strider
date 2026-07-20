package semantic

import "testing"

func TestDocCommentPeriod(t *testing.T) {
	assertStyleCheck(t, "doc-comment-period")
}

func TestDocCommentPeriodDeduplicatesGroupedDeclarationDocs(t *testing.T) {
	root := analysisModule(t, `// Package sample demonstrates grouped declarations.
package sample

// Values are exported constants
const (
	First = 1
	Second = 2
)
`)
	registry, err := newRegistry([]string{
		"doc-comment-period",
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
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want one for the shared comment: %#v", len(diagnostics), diagnostics)
	}
}
