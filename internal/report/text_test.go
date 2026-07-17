package report

import (
	"bytes"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/ui"
)

func TestTextRendersSourceAnnotationAndSummary(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	filename := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(filename, []byte("package p\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics := []diagnostic.Diagnostic{
		{
			Code: "no-init",
			Message: "avoid package initialization",
			Severity: diagnostic.SeverityWarning,
			File: filename,
			Start: token.Position{Filename: filename, Line: 2, Column: 1},
			End: token.Position{Filename: filename, Line: 2, Column: 15},
			Notes: []diagnostic.Note{{Message: "move initialization into an explicit function"}},
		},
	}

	var output bytes.Buffer
	if err := Text(&output, diagnostics, ui.ColorAlways); err != nil {
		t.Fatal(err)
	}
	for _, wanted := range[]string{
		"\x1b[",
		"warning",
		"[no-init]",
		"┌─",
		"func init() {}",
		"^^^^^^^^^^^^^^",
		"  \x1b[1;36m│",
		"note",
		"found 1 issue: 1 warning",
	} {
		if !strings.Contains(output.String(), wanted) {
			t.Fatalf("output missing %q:\n%s", wanted, output.String())
		}
	}
}

func TestTextNeverDoesNotEmitANSI(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	var output bytes.Buffer
	if err := Text(
		&output,
		[]diagnostic.Diagnostic{
			{
				Code: "example",
				Message: "plain",
				Severity: diagnostic.SeverityNote,
				File: "missing.go",
				Start: token.Position{Line: 3, Column: 2},
			},
		},
		ui.ColorNever,
	); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("plain output contains ANSI escapes: %q", output.String())
	}
}

func TestMarkerWidthUsesRuneColumnsFromBytePositions(t *testing.T) {
	item := diagnostic.Diagnostic{
		Start: token.Position{Line: 1, Column: 4},
		End: token.Position{Line: 1, Column: 7},
	}
	if got := markerWidth(item, "é abc", item.Start.Column); got != 3 {
		t.Fatalf("marker width %d; want 3", got)
	}
}
