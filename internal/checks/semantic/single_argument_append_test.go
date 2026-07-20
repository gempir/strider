package semantic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func TestSingleArgumentAppendReportsBuiltinCall(t *testing.T) {
	root := analysisModule(t, `package sample

func unchanged(source []int) []int {
	return append(source)
}
`)
	registry, err := newRegistry([]string{
		"single-argument-append",
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
	if len(diagnostics[0].Fixes) != 1 || !diagnostics[0].Fixes[0].Automatic || diagnostics[0].Fixes[0].Safety != diagnostic.Safe || len(diagnostics[0].Fixes[0].Edits) != 1 {
		t.Fatalf("automatic fix = %#v", diagnostics[0].Fixes)
	}
	contents, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	edit := diagnostics[0].Fixes[0].Edits[0]
	if got := string(contents[edit.Start:edit.End]); got != "append" {
		t.Fatalf("fix deletes %q, want append", got)
	}
}

func TestSingleArgumentAppendDoesNotFixCommentedCall(t *testing.T) {
	root := analysisModule(t, `package sample

func unchanged(source []int) []int {
	return append /* keep */ (source)
}
`)
	registry, err := newRegistry([]string{
		"single-argument-append",
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
	if len(diagnostics[0].Fixes) != 0 {
		t.Fatalf("commented diagnostics = %#v", diagnostics)
	}
}
