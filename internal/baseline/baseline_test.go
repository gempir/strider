package baseline

import (
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func TestLooseBaselineSurvivesLineMovementAndUsesCounts(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "lint-baseline.toml")
	diagnostics := []diagnostic.Diagnostic{
		item(filepath.Join(root, "main.go"), "no-init", "avoid init", 2, 2),
		item(filepath.Join(root, "main.go"), "no-init", "avoid init", 8, 8),
	}
	baseline, err := Generate(path, Loose, diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	if len(baseline.Issues) != 1 || baseline.Issues[0].Count != 2 {
		t.Fatalf("unexpected baseline: %#v", baseline)
	}
	moved := []diagnostic.Diagnostic{
		item(filepath.Join(root, "main.go"), "no-init", "avoid init", 20, 20),
		item(filepath.Join(root, "main.go"), "no-init", "avoid init", 30, 30),
		item(filepath.Join(root, "main.go"), "no-init", "avoid init", 40, 40),
	}
	result, err := Apply(path, baseline, moved)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 || result.Stale != 0 || result.Matched.Issues[0].Count != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestStrictBaselineTracksExactLineRangesAndStaleEntries(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "analysis-baseline.toml")
	baseline, err := Generate(path, Strict, []diagnostic.Diagnostic{
		item(filepath.Join(root, "main.go"), "invalid-regexp", "bad", 4, 5),
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply(path, baseline, []diagnostic.Diagnostic{
		item(filepath.Join(root, "main.go"), "invalid-regexp", "changed", 5, 6),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 || result.Stale != 1 || len(result.Matched.Issues) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestWriteLoadAndBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.toml")
	first := File{Version: Version, Variant: Loose, Issues: []Issue{
		{File: "main.go", Code: "no-init", Message: "avoid init", Count: 1},
	}}
	if err := Write(path, first, false); err != nil {
		t.Fatal(err)
	}
	looseContent, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(looseContent), "start-line") {
		t.Fatalf("loose baseline contains strict fields:\n%s", looseContent)
	}
	second := File{Version: Version, Variant: Strict, Issues: []Issue{
		{File: "main.go", Code: "no-init", StartLine: 2, EndLine: 2},
	}}
	if err := Write(path, second, true); err != nil {
		t.Fatal(err)
	}
	strictContent, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(strictContent), "message =") || strings.Contains(string(strictContent), "count =") {
		t.Fatalf("strict baseline contains loose fields:\n%s", strictContent)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Variant != Strict || len(loaded.Issues) != 1 {
		t.Fatalf("unexpected load: %#v", loaded)
	}
	if _, err := os.Stat(path + ".bkp"); err != nil {
		t.Fatal(err)
	}
}

func item(file, code, message string, start, end int) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		File: file, Code: code, Message: message,
		Start: token.Position{Line: start}, End: token.Position{Line: end},
	}
}
