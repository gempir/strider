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
	baseline, err := Generate(path, Strict, []diagnostic.Diagnostic{item(filepath.Join(root, "main.go"), "invalid-regexp", "bad", 4, 5)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply(path, baseline, []diagnostic.Diagnostic{item(filepath.Join(root, "main.go"), "invalid-regexp", "changed", 5, 6)})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 || result.Stale != 1 || len(result.Matched.Issues) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestApplySelectedPreservesUnselectedEntriesWithoutMarkingThemStale(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "baseline.toml")
	generated, err := Generate(
		path,
		Loose,
		[]diagnostic.Diagnostic{item(filepath.Join(root, "main.go"), "advisory", "old note", 2, 2), item(filepath.Join(root, "main.go"), "critical", "old error", 3, 3)},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApplySelected(
		path,
		generated,
		[]diagnostic.Diagnostic{item(filepath.Join(root, "main.go"), "critical", "old error", 30, 30)},
		map[string]bool{"critical": true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stale != 0 || len(result.Diagnostics) != 0 || len(result.Matched.Issues) != 2 {
		t.Fatalf("unexpected selected result: %#v", result)
	}
	codes := map[string]bool{}
	for _, issue := range result.Matched.Issues {
		codes[issue.Code] = true
	}
	if !codes["advisory"] || !codes["critical"] {
		t.Fatalf("matched baseline lost an inactive code: %#v", result.Matched)
	}
}

func TestApplyCatalogSelectionMakesUnknownCodesStale(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "baseline.toml")
	generated, err := Generate(
		path,
		Loose,
		[]diagnostic.Diagnostic{
			item(filepath.Join(root, "main.go"), "advisory", "old note", 2, 2),
			item(filepath.Join(root, "main.go"), "critical", "old error", 3, 3),
			item(filepath.Join(root, "main.go"), "removed-check", "old finding", 4, 4),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApplyCatalogSelection(
		path,
		generated,
		[]diagnostic.Diagnostic{item(filepath.Join(root, "main.go"), "critical", "old error", 30, 30)},
		map[string]bool{"critical": true},
		map[string]bool{"advisory": true, "critical": true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stale != 1 || len(result.Diagnostics) != 0 || len(result.Matched.Issues) != 2 {
		t.Fatalf("unexpected catalog-aware result: %#v", result)
	}
	for _, issue := range result.Matched.Issues {
		if issue.Code == "removed-check" {
			t.Fatalf("removed check survived catalog-aware pruning: %#v", result.Matched)
		}
	}
}

func TestWriteLoadAndBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.toml")
	first := File{Version: Version, Variant: Loose, Issues: []Issue{{File: "main.go", Code: "no-init", Message: "avoid init", Count: 1}}}
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
	second := File{Version: Version, Variant: Strict, Issues: []Issue{{File: "main.go", Code: "no-init", StartLine: 2, EndLine: 2}}}
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
	return diagnostic.Diagnostic{File: file, Code: code, Message: message, Start: token.Position{Line: start}, End: token.Position{Line: end}}
}
