package report

import (
	"bytes"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
			Start: token.Position{
				Filename: filename,
				Line:     2,
				Column:   1,
			},
			End: token.Position{
				Filename: filename,
				Line:     2,
				Column:   15,
			},
			Notes: []diagnostic.Note{
				{
					Message: "move initialization into an explicit function",
				},
			},
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
		"1 \x1b[38;5;2m│\x1b[0m package p",
		"2 \x1b[38;5;2m│\x1b[0m \x1b[1;33mfunc init() {}\x1b[0m",
		"3 \x1b[38;5;2m│\x1b[0m func run() {}",
		"  \x1b[38;5;2m│",
		"note",
		"1 issue:",
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
	}{
		{
			line:      1,
			wantStart: 1,
			wantEnd:   2,
		},
		{
			line:      2,
			wantStart: 1,
			wantEnd:   3,
		},
		{
			line:      3,
			wantStart: 2,
			wantEnd:   3,
		},
		{
			line:      4,
			wantStart: 0,
			wantEnd:   0,
		},
	} {
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
		Start: token.Position{
			Line:   550,
			Column: 10,
		},
		End: token.Position{
			Line:   550,
			Column: 16,
		},
	}
	var output bytes.Buffer
	if err := Text(&output, []diagnostic.Diagnostic{
		item,
	}, ui.ColorNever); err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{
		"example: broken",
		"    ┌─ " + filename + ":550:10",
		"    │",
		"549 │",
		"550 │ value := broken()",
		"551 │",
	} {
		if !strings.Contains(output.String(), wanted) {
			t.Fatalf("output missing %q:\n%s", wanted, output.String())
		}
	}
}

func TestTextSummaryOnlySuppressesDetailsAndCheckCounts(t *testing.T) {
	diagnostics := []diagnostic.Diagnostic{
		{
			Code:     "note-rule",
			Message:  "detail one",
			Severity: diagnostic.SeverityNote,
		},
		{
			Code:     "error-rule",
			Message:  "detail two",
			Severity: diagnostic.SeverityError,
		},
	}
	var output bytes.Buffer
	if err := TextWithOptions(&output, diagnostics, ui.ColorNever, TextOptions{
		SummaryOnly: true,
	}); err != nil {
		t.Fatal(err)
	}
	want := "error-rule  1\nnote-rule   1\n2 issues: 1 error, 1 note\n"
	if output.String() != want {
		t.Fatalf("summary-only output = %q", output.String())
	}
}

func TestTextRunStatisticsAppearBeforeIssueSummary(t *testing.T) {
	diagnostics := []diagnostic.Diagnostic{
		{
			Code:     "error-rule",
			Message:  "detail",
			Severity: diagnostic.SeverityError,
		},
	}
	var output bytes.Buffer
	if err := TextWithOptions(&output, diagnostics, ui.ColorNever, TextOptions{
		Statistics: &RunStatistics{
			Files:    50,
			Checks:   202,
			Duration: 7500 * time.Microsecond,
		},
	}); err != nil {
		t.Fatal(err)
	}
	want := "error-rule  1\nChecked 50 files in 8ms. Ran 202 checks.\n1 issue: 1 error\n"
	if !strings.HasSuffix(output.String(), want) {
		t.Fatalf("statistics output = %q, want suffix %q", output.String(), want)
	}
}

func TestTextRunStatisticsSupportSummaryOnlyAndSingularCounts(t *testing.T) {
	var output bytes.Buffer
	if err := TextWithOptions(&output, nil, ui.ColorNever, TextOptions{
		SummaryOnly: true,
		Statistics: &RunStatistics{
			Files:    1,
			Checks:   1,
			Duration: 400 * time.Microsecond,
		},
	}); err != nil {
		t.Fatal(err)
	}
	want := "Checked 1 file in 0ms. Ran 1 check.\n0 issues\n"
	if output.String() != want {
		t.Fatalf("summary-only statistics = %q, want %q", output.String(), want)
	}
}

func TestTextRunStatisticsAreMutedWhenColorIsEnabled(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	var output bytes.Buffer
	if err := TextWithOptions(&output, nil, ui.ColorAlways, TextOptions{
		Statistics: &RunStatistics{
			Files:    2,
			Checks:   3,
			Duration: time.Millisecond,
		},
	}); err != nil {
		t.Fatal(err)
	}
	want := "\x1b[2mChecked 2 files in 1ms. Ran 3 checks.\x1b[0m\n"
	if !strings.HasPrefix(output.String(), want) {
		t.Fatalf("colored statistics = %q, want prefix %q", output.String(), want)
	}
}

func TestTextRendersCleanSummary(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	var output bytes.Buffer
	if err := Text(&output, nil, ui.ColorAlways); err != nil {
		t.Fatal(err)
	}
	if output.String() != "\x1b[38;5;10m0 issues\x1b[0m\n" {
		t.Fatalf("clean summary = %q", output.String())
	}
}

func TestTextSummarizesFindingsByCheck(t *testing.T) {
	diagnostics := []diagnostic.Diagnostic{
		{
			Code:     "second",
			Message:  "one",
			Severity: diagnostic.SeverityWarning,
			File:     "missing.go",
		},
		{
			Code:     "first",
			Message:  "one",
			Severity: diagnostic.SeverityError,
			File:     "missing.go",
		},
		{
			Code:     "first",
			Message:  "two",
			Severity: diagnostic.SeverityError,
			File:     "missing.go",
		},
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
	if !strings.HasSuffix(text, "3 issues: 2 errors, 1 warning\n") {
		t.Fatalf("issue summary is not the final line:\n%s", text)
	}
}

func TestSummaryColorsEachSeveritySegment(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	palette := ui.NewPalette(&bytes.Buffer{}, ui.ColorAlways)
	got := summary(
		make([]diagnostic.Diagnostic, 221),
		map[diagnostic.Severity]int{
			diagnostic.SeverityError:   214,
			diagnostic.SeverityWarning: 7,
		},
		map[fixability]int{
			safelyFixable:   4,
			unsafelyFixable: 2,
		},
		palette,
	)
	want := "\x1b[1;37m221 issues: \x1b[0m" + "\x1b[1;31m214 errors\x1b[0m" + "\x1b[1;37m, \x1b[0m" + "\x1b[1;33m7 warnings\x1b[0m" + "\x1b[1;37m, \x1b[0m" + "\x1b[38;5;10m4 fixable\x1b[0m" + "\x1b[1;37m, \x1b[0m" + "\x1b[1;35m2 unsafe fixable\x1b[0m"
	if got != want {
		t.Fatalf("colored summary = %q, want %q", got, want)
	}
}

func TestTextMarksAutomaticFixesWithoutRenderingHelp(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	diagnostics := []diagnostic.Diagnostic{
		{
			Code:     "safe-rule",
			Message:  "safe",
			Severity: diagnostic.SeverityError,
			File:     "missing.go",
			Fixes: []diagnostic.Fix{
				{
					Message:   "apply safe fix",
					Safety:    diagnostic.Safe,
					Automatic: true,
				},
			},
		},
		{
			Code:     "unsafe-rule",
			Message:  "unsafe",
			Severity: diagnostic.SeverityError,
			File:     "missing.go",
			Fixes: []diagnostic.Fix{
				{
					Message:   "apply unsafe fix",
					Safety:    diagnostic.Unsafe,
					Automatic: true,
				},
			},
		},
		{
			Code:     "plain-rule",
			Message:  "plain",
			Severity: diagnostic.SeverityWarning,
			File:     "missing.go",
		},
	}
	var output bytes.Buffer
	if err := Text(&output, diagnostics, ui.ColorAlways); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, unwanted := range []string{
		"apply safe fix",
		"apply unsafe fix",
		"help:",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("output contains removed fix help %q:\n%s", unwanted, text)
		}
	}
	for _, wanted := range []string{
		"\x1b[1;31msafe-rule\x1b[0m\x1b[38;5;10m*\x1b[0m:",
		"\x1b[1;31munsafe-rule\x1b[0m\x1b[1;35m*\x1b[0m:",
		"\x1b[1;31msafe-rule\x1b[0m\x1b[38;5;10m*\x1b[0m    ",
		"\x1b[1;31munsafe-rule\x1b[0m\x1b[1;35m*\x1b[0m  ",
		"\x1b[38;5;10m1 fixable\x1b[0m",
		"\x1b[1;35m1 unsafe fixable\x1b[0m",
	} {
		if !strings.Contains(text, wanted) {
			t.Fatalf("output missing %q:\n%s", wanted, text)
		}
	}
	plainSummary := summary(
		diagnostics,
		map[diagnostic.Severity]int{
			diagnostic.SeverityError:   2,
			diagnostic.SeverityWarning: 1,
		},
		map[fixability]int{
			safelyFixable:   1,
			unsafelyFixable: 1,
		},
		ui.NewPalette(&bytes.Buffer{}, ui.ColorNever),
	)
	if plainSummary != "3 issues: 2 errors, 1 warning, 1 fixable, 1 unsafe fixable" {
		t.Fatalf("plain summary = %q", plainSummary)
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
				Code:     "example",
				Message:  "plain",
				Severity: diagnostic.SeverityNote,
				File:     "missing.go",
				Start: token.Position{
					Line:   3,
					Column: 2,
				},
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

func TestSourceSpanUsesByteColumns(t *testing.T) {
	item := diagnostic.Diagnostic{
		Start: token.Position{
			Line:   1,
			Column: 4,
		},
		End: token.Position{
			Line:   1,
			Column: 7,
		},
	}
	start, end := sourceSpan(item, "é abc")
	if start != 3 || end != 6 {
		t.Fatalf("source span = (%d, %d), want (3, 6)", start, end)
	}
}
