package baseline

import (
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func TestStrictBaselineTracksExactLineRangesAndStaleEntries(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "analysis-baseline.toml")
	baseline, err := Generate(path, []diagnostic.Diagnostic{item(filepath.Join(root, "main.go"), "invalid-regexp", "bad", 4, 5)})
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
		[]diagnostic.Diagnostic{item(filepath.Join(root, "main.go"), "advisory", "old note", 2, 2), item(filepath.Join(root, "main.go"), "critical", "old error", 3, 3)},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApplySelected(
		path,
		generated,
		[]diagnostic.Diagnostic{item(filepath.Join(root, "main.go"), "critical", "old error", 3, 3)},
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
		[]diagnostic.Diagnostic{item(filepath.Join(root, "main.go"), "critical", "old error", 3, 3)},
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

func TestWriteReplacesAndLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.toml")
	first := File{Version: Version, Variant: Strict, Issues: []Issue{{File: "main.go", Code: "no-init", StartLine: 1, EndLine: 1}}}
	if err := Write(path, first); err != nil {
		t.Fatal(err)
	}
	second := File{Version: Version, Variant: Strict, Issues: []Issue{{File: "main.go", Code: "no-init", StartLine: 2, EndLine: 2}}}
	if err := Write(path, second); err != nil {
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
}

func TestLoadRejectsLooseBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.toml")
	content := "version = 1\nvariant = \"loose\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), `baseline variant must be "strict"`) {
		t.Fatalf("Load error = %v, want strict variant error", err)
	}
}

func item(file, code, message string, start, end int) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{File: file, Code: code, Message: message, Start: token.Position{Line: start}, End: token.Position{Line: end}}
}
