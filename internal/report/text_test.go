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
	if err := os.WriteFile(filename, []byte("package p\nfunc init() {}\nfunc run() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics := []diagnostic.Diagnostic{
		{
			Code:     "no-init",
			Message:  "avoid package initialization",
			Severity: diagnostic.SeverityWarning,
			File:     filename,
			Start:    token.Position{Filename: filename, Line: 2, Column: 1},
			End:      token.Position{Filename: filename, Line: 2, Column: 15},
			Notes:    []diagnostic.Note{{Message: "move initialization into an explicit function"}},
		},
	}

	var output bytes.Buffer
	if err := Text(&output, diagnostics, ui.ColorAlways); err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{
		"\x1b[",
		"\x1b[1;33mno-init\x1b[0m",
		"┌─",
		"1 \x1b[1;35m│\x1b[0m package p",
		"2 \x1b[1;35m│\x1b[0m \x1b[1;33mfunc init() {}\x1b[0m",
		"3 \x1b[1;35m│\x1b[0m func run() {}",
		"  \x1b[1;35m│",
		"note",
		"found 1 issue:",
		"no-init",
		"1",
	} {
		if !strings.Contains(output.String(), wanted) {
			t.Fatalf("output missing %q:\n%s", wanted, output.String())
		}
	}
}

func TestSourceContextClampsAtFileEdges(t *testing.T) {
	for _, test := range []struct {
		line      int
		wantStart int
		wantEnd   int
	}{{line: 1, wantStart: 1, wantEnd: 2}, {line: 2, wantStart: 1, wantEnd: 3}, {line: 3, wantStart: 2, wantEnd: 3}, {line: 4, wantStart: 0, wantEnd: 0}} {
		start, end := sourceContext(test.line, 3)
		if start != test.wantStart || end != test.wantEnd {
			t.Errorf("sourceContext(%d, 3) = (%d, %d), want (%d, %d)", test.line, start, end, test.wantStart, test.wantEnd)
		}
	}
}

func TestTextAlignsLocationConnectorWithSourceGutter(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "main.go")
	lines := strings.Repeat("\n", 549) + "value := broken()\n"
	if err := os.WriteFile(filename, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	item := diagnostic.Diagnostic{
		Code:     "example",
		Message:  "broken",
		Severity: diagnostic.SeverityError,
		File:     filename,
		Start:    token.Position{Line: 550, Column: 10},
		End:      token.Position{Line: 550, Column: 16},
	}
	var output bytes.Buffer
	if err := Text(&output, []diagnostic.Diagnostic{item}, ui.ColorNever); err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{"example: broken", "    ┌─ " + filename + ":550:10", "    │", "549 │", "550 │ value := broken()", "551 │"} {
		if !strings.Contains(output.String(), wanted) {
			t.Fatalf("output missing %q:\n%s", wanted, output.String())
		}
	}
}

func TestTextSummaryOnlySuppressesDetailsAndCheckCounts(t *testing.T) {
	diagnostics := []diagnostic.Diagnostic{
		{Code: "note-rule", Message: "detail one", Severity: diagnostic.SeverityNote},
		{Code: "error-rule", Message: "detail two", Severity: diagnostic.SeverityError},
	}
	var output bytes.Buffer
	if err := TextWithOptions(&output, diagnostics, ui.ColorNever, TextOptions{SummaryOnly: true}); err != nil {
		t.Fatal(err)
	}
	want := "error-rule  1\nnote-rule   1\nfound 2 issues: 1 error, 1 note\n"
	if output.String() != want {
		t.Fatalf("summary-only output = %q", output.String())
	}
}

func TestTextRendersCleanSummary(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	var output bytes.Buffer
	if err := Text(&output, nil, ui.ColorAlways); err != nil {
		t.Fatal(err)
	}
	if output.String() != "\x1b[1;32mfound 0 issues\x1b[0m\n" {
		t.Fatalf("clean summary = %q", output.String())
	}
}

func TestTextSummarizesFindingsByCheck(t *testing.T) {
	diagnostics := []diagnostic.Diagnostic{
		{Code: "second", Message: "one", Severity: diagnostic.SeverityWarning, File: "missing.go"},
		{Code: "first", Message: "one", Severity: diagnostic.SeverityError, File: "missing.go"},
		{Code: "first", Message: "two", Severity: diagnostic.SeverityError, File: "missing.go"},
	}
	var output bytes.Buffer
	if err := Text(&output, diagnostics, ui.ColorNever); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	first := strings.Index(text, "\nfirst   2")
	second := strings.Index(text, "\nsecond  1")
	if first < 0 || second < 0 || first >= second || strings.Contains(text, "Check summary") || strings.Contains(text, "checks broken") {
		t.Fatalf("unexpected check summary:\n%s", text)
	}
	if !strings.HasSuffix(text, "found 3 issues: 2 errors, 1 warning\n") {
		t.Fatalf("issue summary is not the final line:\n%s", text)
	}
}

func TestSummaryColorsEachSeveritySegment(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	palette := ui.NewPalette(&bytes.Buffer{}, ui.ColorAlways)
	got := summary(make([]diagnostic.Diagnostic, 221), map[diagnostic.Severity]int{diagnostic.SeverityError: 214, diagnostic.SeverityWarning: 7}, palette)
	want := "\x1b[1;37mfound 221 issues: \x1b[0m" + "\x1b[1;31m214 errors\x1b[0m" + "\x1b[1;37m, \x1b[0m" + "\x1b[1;33m7 warnings\x1b[0m"
	if got != want {
		t.Fatalf("colored summary = %q, want %q", got, want)
	}
}

func TestTextNeverDoesNotEmitANSI(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	var output bytes.Buffer
	if err := Text(
		&output,
		[]diagnostic.Diagnostic{{Code: "example", Message: "plain", Severity: diagnostic.SeverityNote, File: "missing.go", Start: token.Position{Line: 3, Column: 2}}},
		ui.ColorNever,
	); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("plain output contains ANSI escapes: %q", output.String())
	}
}

func TestSourceSpanUsesByteColumns(t *testing.T) {
	item := diagnostic.Diagnostic{Start: token.Position{Line: 1, Column: 4}, End: token.Position{Line: 1, Column: 7}}
	start, end := sourceSpan(item, "é abc")
	if start != 3 || end != 6 {
		t.Fatalf("source span = (%d, %d), want (3, 6)", start, end)
	}
}
