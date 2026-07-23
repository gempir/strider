package report

import (
	"bytes"
	"encoding/json"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func TestJSONGoldenAndRoundTrip(t *testing.T) {
	diagnostics := []diagnostic.Diagnostic{
		{
			Code:     "example-check",
			Message:  `found <tag> & "quoted" value`,
			Severity: diagnostic.SeverityError,
			File:     "nested/example.go",
			Start: token.Position{
				Filename: "nested/example.go",
				Offset:   7,
				Line:     2,
				Column:   3,
			},
			End: token.Position{
				Filename: "nested/example.go",
				Offset:   12,
				Line:     2,
				Column:   8,
			},
			Notes: []diagnostic.Note{
				{
					Message: "declared here",
					Start: token.Position{
						Filename: "nested/example.go",
						Offset:   0,
						Line:     1,
						Column:   1,
					},
					End: token.Position{
						Filename: "nested/example.go",
						Offset:   4,
						Line:     1,
						Column:   5,
					},
				},
			},
			Fixes: []diagnostic.Fix{
				{
					Message:   "replace the value",
					Safety:    diagnostic.Safe,
					Automatic: true,
					Edits: []diagnostic.TextEdit{
						{
							Start:   7,
							End:     12,
							OldText: "value",
							NewText: "other",
						},
						{
							Start:   12,
							End:     12,
							NewText: "!",
						},
					},
				},
				{
					Message: "review manually",
					Safety:  diagnostic.Unsafe,
				},
			},
		},
		{
			Code:     "empty-fields",
			Message:  "no notes or fixes",
			Severity: diagnostic.SeverityNote,
			File:     "empty.go",
			Start: token.Position{
				Filename: "empty.go",
				Line:     1,
				Column:   1,
			},
			End: token.Position{
				Filename: "empty.go",
				Line:     1,
				Column:   1,
			},
		},
	}
	var output bytes.Buffer
	if err := JSON(&output, diagnostics); err != nil {
		t.Fatal(err)
	}
	_, testFile, _, _ := runtime.Caller(0)
	goldenPath := filepath.Join(filepath.Dir(testFile), "testdata", "json.golden")
	if os.Getenv("STRIDER_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, output.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), want) {
		t.Fatalf("JSON differs from testdata/json.golden\ngot:\n%s\nwant:\n%s", output.Bytes(), want)
	}
	var decoded []diagnostic.Diagnostic
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, diagnostics) {
		t.Fatalf("round trip = %#v, want %#v", decoded, diagnostics)
	}
}

func TestJSONZeroFindingsIsAnEmptyArray(t *testing.T) {
	var output bytes.Buffer
	if err := JSON(&output, []diagnostic.Diagnostic{}); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "[]\n"; got != want {
		t.Fatalf("zero findings = %q, want %q", got, want)
	}
}
