package analyze

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSA1000ReportsConstantInvalidRegexps(t *testing.T) {
	root := analysisModule(t, `package sample

import rx "regexp"

const invalid = "["

func check(pattern string) {
	rx.MustCompile(invalid)
	local := "("
	rx.Compile(local)
	rx.MatchString("[a-", "value")
	compile := rx.Compile
	compile("[")
	rx.Compile(pattern)
}
`)
	registry, err := NewRegistry([]string{"sa1000"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 4 {
		t.Fatalf("got %d diagnostics, want 4: %#v", len(diagnostics), diagnostics)
	}
	for _, item := range diagnostics {
		if item.Code != "SA1000" || !strings.Contains(item.Message, "error parsing regexp") {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
		if item.Start.Filename != "main.go" {
			t.Fatalf("unexpected display path: %q", item.Start.Filename)
		}
	}
}

func TestSA1000AcceptsValidAndDynamicRegexps(t *testing.T) {
	root := analysisModule(t, `package sample

import "regexp"

func check(pattern string) {
	regexp.MustCompile("[a-z]+")
	regexp.Compile(pattern)
}
`)
	registry, err := NewRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
}

func TestSA1001ReportsInvalidDirectTemplates(t *testing.T) {
	root := analysisModule(t, `package sample

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
`)
	registry, err := NewRegistry([]string{"SA1001"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
	for _, item := range diagnostics {
		validMessage := strings.Contains(item.Message, "unexpected") ||
			strings.Contains(item.Message, "bad character")
		if item.Code != "SA1001" || !validMessage {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}

func TestSA1002ReportsInvalidConstantTimeLayouts(t *testing.T) {
	root := analysisModule(t, `package sample

import "time"

const invalidLayout = "12345"

func check(value string) {
	time.Parse(invalidLayout, value)
	local := "12345"
	time.Parse(local, value)
	time.Parse("2006-01-02", value)
	time.Parse(time.RFC3339Nano, value)
}
`)
	registry, err := NewRegistry([]string{"SA1002"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
	for _, item := range diagnostics {
		if item.Code != "SA1002" || !strings.Contains(item.Message, "parsing time") {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}

func TestRegistryRejectsUnknownRule(t *testing.T) {
	if _, err := NewRegistry([]string{"SA9999"}); err == nil ||
		!strings.Contains(err.Error(), "SA9999") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func analysisModule(t *testing.T, source string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module example.com/analysis\n\ngo 1.26\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(
		func() {
			_ = os.Chdir(previous)
		},
	)
	return root
}
