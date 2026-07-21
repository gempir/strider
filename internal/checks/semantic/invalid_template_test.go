package semantic

import (
	"strings"
	"testing"
)

func TestInvalidTemplateReportsInvalidDirectTemplates(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	htmltemplate "html/template"
	texttemplate "text/template"
)

const invalid = "{{.Name}} {{.LastName}"

func check() {
	texttemplate.New("").Parse(invalid)
	htmltemplate.New("").Parse(invalid)
	texttemplate.New("").Parse("{{missingFunction}}")
	template := texttemplate.New("")
	template.Parse(invalid)
	texttemplate.New("").Delims("[[", "]]").Parse("{{broken-}}")
}
`,
	)
	registry, err := newRegistry([]string{
		"invalid-template",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnosticGolden(t, diagnostics)
	for _, item := range diagnostics {
		validMessage := strings.Contains(item.Message, "unexpected") || strings.Contains(item.Message, "bad character")
		if item.Code != "invalid-template" || !validMessage {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}
