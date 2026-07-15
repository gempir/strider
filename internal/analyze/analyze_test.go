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
	if len(diagnostics) != 3 {
		t.Fatalf("got %d diagnostics, want 3: %#v", len(diagnostics), diagnostics)
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
