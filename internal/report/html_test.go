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
	diagnostics := []diagnostic.Diagnostic{{
		Code: "example-rule", Message: "value <script>alert(1)</script>",
		Severity: diagnostic.SeverityError, File: filename,
		Start: token.Position{Line: 2, Column: 5}, End: token.Position{Line: 2, Column: 10},
		Notes: []diagnostic.Note{{Message: "a useful note"}},
		Fixes: []diagnostic.Fix{{Message: "replace the value", Safety: diagnostic.Safe}},
	}}

	var output bytes.Buffer
	if err := HTML(&output, "Lint <report>", diagnostics); err != nil {
		t.Fatal(err)
	}
	page := output.String()
	for _, wanted := range []string{
		"<!doctype html>", "Lint &lt;report&gt;", "example-rule", "data-severity=\"error\"",
		"var value = 1 &lt; 2", "a useful note", "Fix (safe):", "id=\"search\"",
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
