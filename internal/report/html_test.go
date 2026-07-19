package report

import (
	"bytes"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func TestHTMLRendersSelfContainedSearchableReport(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(filename, []byte("package p\nvar value = 1 < 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics := []diagnostic.Diagnostic{
		{
			Code:     "example-rule",
			Message:  "value <script>alert(1)</script>",
			Severity: diagnostic.SeverityError,
			File:     filename,
			Start: token.Position{
				Line:   2,
				Column: 5,
			},
			End: token.Position{
				Line:   2,
				Column: 10,
			},
			Notes: []diagnostic.Note{
				{
					Message: "a useful note",
				},
			},
			Fixes: []diagnostic.Fix{
				{
					Message: "replace the value",
					Safety:  diagnostic.Safe,
				},
			},
		},
	}

	var output bytes.Buffer
	if err := HTML(&output, "Lint <report>", diagnostics); err != nil {
		t.Fatal(err)
	}
	page := output.String()
	for _, wanted := range []string{
		"<!doctype html>",
		"Lint &lt;report&gt;",
		"example-rule",
		"data-severity=\"error\"",
		"<mark>value</mark>",
		"a useful note",
		"Fix (safe):",
		"id=\"search\"",
	} {
		if !strings.Contains(page, wanted) {
			t.Fatalf("HTML output missing %q:\n%s", wanted, page)
		}
	}
	if strings.Contains(page, "<script>alert(1)</script>") {
		t.Fatal("diagnostic message was not HTML escaped")
	}
}

func TestHTMLRendersEmptyReport(t *testing.T) {
	var output bytes.Buffer
	if err := HTML(&output, "Analysis report", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "No diagnostics found.") {
		t.Fatalf("empty report missing clean state: %s", output.String())
	}
}

func TestHTMLRendersOperationTimings(t *testing.T) {
	var output bytes.Buffer
	options := HTMLOptions{
		Title: "Benchmark report",
		Timings: []HTMLTiming{
			{
				Name:       "format",
				DurationMS: 14,
			},
			{
				Name:       "lint",
				DurationMS: 287,
			},
			{
				Name:       "analyze",
				DurationMS: 903,
			},
		},
	}
	if err := HTMLWithOptions(&output, options, nil); err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{
		"Operation timings",
		"format",
		"14 <small>ms</small>",
		"903 <small>ms</small>",
	} {
		if !strings.Contains(output.String(), wanted) {
			t.Fatalf("HTML output missing timing %q: %s", wanted, output.String())
		}
	}
}

func TestHTMLResolvesRelativeSourcesAgainstRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package p\nvar answer = 42\nfunc use() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics := []diagnostic.Diagnostic{
		{
			Code:     "example",
			Message:  "highlight the answer",
			Severity: diagnostic.SeverityWarning,
			File:     "main.go",
			Start: token.Position{
				Line:   2,
				Column: 5,
			},
			End: token.Position{
				Line:   2,
				Column: 11,
			},
		},
	}
	var output bytes.Buffer
	if err := HTMLWithOptions(&output, HTMLOptions{
		Title:      "Corpus",
		SourceRoot: root,
	}, diagnostics); err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{
		"package p",
		"<mark>answer</mark>",
		"func use() {}",
	} {
		if !strings.Contains(output.String(), wanted) {
			t.Fatalf("HTML output missing source context %q: %s", wanted, output.String())
		}
	}
}

func TestHTMLLimitsDetailsButSummarizesAllDiagnostics(t *testing.T) {
	diagnostics := []diagnostic.Diagnostic{
		{
			Code:     "common-rule",
			Message:  "common one",
			Severity: diagnostic.SeverityWarning,
		},
		{
			Code:     "common-rule",
			Message:  "common two",
			Severity: diagnostic.SeverityWarning,
		},
		{
			Code:     "rare-rule",
			Message:  "rare one",
			Severity: diagnostic.SeverityError,
		},
	}
	var output bytes.Buffer
	if err := HTMLWithOptions(&output, HTMLOptions{
		Title:          "Limited report",
		MaxDiagnostics: 2,
	}, diagnostics); err != nil {
		t.Fatal(err)
	}
	page := output.String()
	if got := strings.Count(page, `<details class="diagnostic"`); got != 2 {
		t.Fatalf("rendered %d diagnostic details, want 2", got)
	}
	for _, wanted := range []string{
		"Showing 2 of 3 detailed findings",
		"The summary includes all 3 findings",
		"common-rule</code><i style=\"width:100%\"></i></button></td><td>2",
		"rare-rule</code><i style=\"width:50%\"></i></button></td><td>1",
		"common one",
		"rare one",
	} {
		if !strings.Contains(page, wanted) {
			t.Fatalf("HTML output missing %q: %s", wanted, page)
		}
	}
	if strings.Contains(page, "common two") {
		t.Fatal("report rendered a diagnostic beyond the detail limit")
	}
	if strings.Index(page, "data-rule=\"common-rule\"") > strings.Index(page, "data-rule=\"rare-rule\"") {
		t.Fatal("rule summary was not sorted by descending finding count")
	}
}
