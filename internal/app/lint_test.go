package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintJSONAndExitCode(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte("package p\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{
		"check",
		"--format",
		"json",
		"--minimum-severity",
		"note",
		"--only",
		"no-init",
		root,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings {
		t.Fatalf("exit %d, stderr %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"code": "no-init"`) {
		t.Fatalf("unexpected JSON: %s", stdout.String())
	}
}

func TestLintHTMLAndExitCode(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte("package p\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{
		"check",
		"--format",
		"html",
		"--minimum-severity",
		"note",
		"--only",
		"no-init",
		root,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings {
		t.Fatalf("exit %d, stderr %s", code, stderr.String())
	}
	for _, wanted := range []string{
		"<!doctype html>",
		"Strider check report",
		"no-init",
		"func <mark>init</mark>() {}",
	} {
		if !strings.Contains(stdout.String(), wanted) {
			t.Fatalf("HTML output missing %q: %s", wanted, stdout.String())
		}
	}
}

func TestLintWithoutPathsScansCurrentDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package p\nfunc init() {}\n"), 0o600); err != nil {
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
	code := runCLI([]string{
		"check",
		"--minimum-severity",
		"note",
		"--only",
		"no-init",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings || !strings.Contains(stdout.String(), "no-init") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestLintListsCompleteRegistry(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{
		"check",
		"--minimum-severity",
		"note",
		"--list-checks",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess {
		t.Fatalf("exit %d, stderr %s", code, stderr.String())
	}
	_, marshalListed := listedSeverity(stdout.String(), "marshal-receiver")
	_, splitCheckListed := listedSeverity(stdout.String(), "redundant-final-return")
	_, removedFormatterCheckListed := listedSeverity(stdout.String(), "multiline-if-init")
	if !marshalListed || !splitCheckListed || removedFormatterCheckListed {
		t.Fatalf("complete registry is missing extended checks: %s", stdout.String())
	}
}
