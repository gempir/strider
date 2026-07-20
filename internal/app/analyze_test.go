package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeInvalidRegexpJSONAndExitCode(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/analyzeapp\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package sample\nimport \"regexp\"\nvar _ = regexp.MustCompile(\"[\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		restoreWorkingDirectory(t, previous)
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"check",
		"--format",
		"json",
		"--only",
		"invalid-regexp",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"code": "invalid-regexp"`) {
		t.Fatalf("unexpected JSON: %s", stdout.String())
	}
}

func TestAnalyzeListsChecks(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"check",
		"--list-checks",
	}, strings.NewReader(""), &stdout, &stderr)
	severity, listed := listedSeverity(stdout.String(), "invalid-regexp")
	if code != exitSuccess || !listed || severity != "error" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}
