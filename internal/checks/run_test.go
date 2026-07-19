package checks

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/workspace"
)

func TestRunSharesCSTBetweenFormattingAndSyntaxChecks(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "sample.go")
	original := []byte("package sample\nfunc init(){println(\"x\")}\n")
	if err := os.WriteFile(filename, original, 0o600); err != nil {
		t.Fatal(err)
	}
	shared, err := workspace.Open([]string{
		filename,
	}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"format",
			"no-init",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(shared, registry, RunOptions{
		Formatter:         formatter.DefaultOptions(),
		CollectCandidates: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(result.Diagnostics), 2; got != want {
		t.Fatalf("diagnostic count = %d, want %d: %#v", got, want, result.Diagnostics)
	}
	if result.Diagnostics[0].Code != "format" || result.Diagnostics[1].Code != "no-init" {
		t.Fatalf("diagnostic codes = %q, %q", result.Diagnostics[0].Code, result.Diagnostics[1].Code)
	}
	candidate, ok := result.Candidates[filename]
	if !ok || !candidate.Changed {
		t.Fatal("missing changed formatting candidate")
	}
	contents, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contents, original) {
		t.Fatal("read-only check modified source")
	}
}

func TestRunCSTOnlyDoesNotRequireGoPackage(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "standalone.go")
	if err := os.WriteFile(filename, []byte("package standalone\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shared, err := workspace.Open([]string{
		filename,
	}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"no-init",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(shared, registry, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(result.Diagnostics), 1; got != want {
		t.Fatalf("diagnostic count = %d, want %d", got, want)
	}
}

func TestRunFormatIgnoreFastPathDoesNotParse(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "ignored.go")
	if err := os.WriteFile(filename, []byte("//strider:format-ignore\nthis is intentionally not Go\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shared, err := workspace.Open([]string{
		filename,
	}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"format",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(shared, registry, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 || len(result.Candidates) != 0 {
		t.Fatalf("ignored result = %#v", result)
	}
}
