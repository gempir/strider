//strider:ignore-file cognitive-complexity,cyclomatic-complexity
package checks

import (
	"go/token"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func assertDiagnosticContract(t *testing.T, sources map[string][]byte, diagnostics []diagnostic.Diagnostic) {
	t.Helper()
	for _, item := range diagnostics {
		source, ok := sources[item.File]
		if !ok {
			t.Errorf("%s: diagnostic file has no source fixture", item.File)
			continue
		}
		if item.Code == "" || item.Message == "" || !diagnostic.ValidSeverity(item.Severity) {
			t.Errorf("%s: incomplete diagnostic identity: %#v", item.Code, item)
		}
		if item.Start.Offset < 0 || item.End.Offset < item.Start.Offset || item.End.Offset > len(source) {
			t.Errorf("%s: invalid diagnostic range [%d,%d) for %d bytes", item.Code, item.Start.Offset, item.End.Offset, len(source))
		}
		if item.Start.Line < 1 || item.Start.Column < 1 || item.End.Line < 1 || item.End.Column < 1 {
			t.Errorf("%s: invalid line/column positions: %v - %v", item.Code, item.Start, item.End)
		}
		if item.Start.Filename != item.File || item.End.Filename != item.File {
			t.Errorf("%s: position filenames do not match diagnostic file", item.Code)
		}
		automatic := 0
		for _, fix := range item.Fixes {
			if fix.Message == "" || !diagnostic.ValidSafety(fix.Safety) {
				t.Errorf("%s: invalid fix metadata: %#v", item.Code, fix)
			}
			if fix.Automatic {
				automatic++
			}
			previousEnd := -1
			for _, edit := range fix.Edits {
				if edit.Start < 0 || edit.End < edit.Start || edit.End > len(source) {
					t.Errorf("%s: invalid edit range [%d,%d)", item.Code, edit.Start, edit.End)
					continue
				}
				if edit.Start < previousEnd {
					t.Errorf("%s: edits are overlapping or out of order", item.Code)
				}
				previousEnd = edit.End
				if edit.OldText != "" && edit.OldText != string(source[edit.Start:edit.End]) {
					t.Errorf("%s: edit OldText %q does not match %q", item.Code, edit.OldText, source[edit.Start:edit.End])
				}
			}
		}
		if automatic > 1 {
			t.Errorf("%s: has %d automatic fix alternatives", item.Code, automatic)
		}
	}
}

func TestDiagnosticContractHelperAcceptsCompleteDiagnostics(t *testing.T) {
	source := []byte("package sample\n")
	complete := diagnostic.Diagnostic{
		Code:     "example",
		Message:  "example",
		Severity: diagnostic.SeverityWarning,
		File:     "sample.go",
		Start:    contractPosition("sample.go", 0),
		End:      contractPosition("sample.go", len("package")),
		Fixes: []diagnostic.Fix{
			{
				Message:   "replace package",
				Safety:    diagnostic.Safe,
				Automatic: true,
				Edits: []diagnostic.TextEdit{
					{
						Start:   0,
						End:     len("package"),
						OldText: "package",
						NewText: "package",
					},
				},
			},
		},
	}
	assertDiagnosticContract(t, map[string][]byte{
		"sample.go": source,
	}, []diagnostic.Diagnostic{
		complete,
	})
}

func contractPosition(filename string, offset int) token.Position {
	return token.Position{
		Filename: filename,
		Offset:   offset,
		Line:     1,
		Column:   offset + 1,
	}
}
