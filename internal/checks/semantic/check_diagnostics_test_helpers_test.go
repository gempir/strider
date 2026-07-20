package semantic

import (
	"strings"
	"testing"
)

func assertCheckDiagnostics(t *testing.T, code, source string, want int, messageFragment string) {
	t.Helper()
	root := analysisModule(t, source)
	registry, err := newRegistry([]string{
		code,
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
	if len(diagnostics) != want {
		t.Fatalf("got %d diagnostics, want %d: %#v", len(diagnostics), want, diagnostics)
	}
	for _, item := range diagnostics {
		if item.Code != code || !strings.Contains(item.Message, messageFragment) {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}
