//strider:ignore-file cognitive-complexity,cyclomatic-complexity
package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/baseline"
)

func TestCheckFixDoesNotApplyBaselinedFinding(t *testing.T) {
	root, filename := checkFixModule(t, "package sample\n\nfunc ready(value bool) bool { return !!value }\n")
	baselinePath := filepath.Join(root, "baseline.toml")
	baseArgs := []string{
		"--no-config",
		"check",
		"--only",
		"double-negation",
		"--baseline",
		baselinePath,
	}
	var stdout, stderr bytes.Buffer
	generate := append(append([]string(nil), baseArgs...), "--generate-baseline", filename)
	if code := runCLI(generate, strings.NewReader(""), &stdout, &stderr); code != exitSuccess || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("generate: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	before, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	apply := append(append([]string(nil), baseArgs...), "--fix", filename)
	if code := runCLI(apply, strings.NewReader(""), &stdout, &stderr); code != exitSuccess || withoutRunStatistics(stdout.String()) != "0 issues\n" || stderr.Len() != 0 {
		t.Fatalf("fix: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	after, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("baselined finding was fixed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestCheckBaselineDoesNotCaptureFormattingDebt(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "main.go")
	baselinePath := filepath.Join(root, "strider-baseline.toml")
	if err := os.WriteFile(filename, []byte("package sample\nfunc init(){}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runCLI(
		[]string{
			"--no-config",
			"check",
			"--only",
			"format,no-init",
			"--minimum-severity",
			"note",
			"--baseline",
			baselinePath,
			"--generate-baseline",
			filename,
		},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitSuccess || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	generated, err := baseline.Load(baselinePath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(generated.Issues), 1; got != want {
		t.Fatalf("baseline issue count = %d, want %d: %#v", got, want, generated.Issues)
	}
	if generated.Issues[0].Code != "no-init" {
		t.Fatalf("baseline captured %q, want no-init", generated.Issues[0].Code)
	}
}

func TestCheckBaselineGenerateApplyIgnoreAndPrune(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "main.go")
	baselinePath := filepath.Join(root, "lint-baseline.toml")
	write := func(source string) {
		t.Helper()
		if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("package p\nfunc init() {}\n")
	run := func(extra ...string) (int, string, string) {
		t.Helper()
		args := []string{
			"check",
			"--minimum-severity",
			"note",
			"--only",
			"no-init",
			"--baseline",
			baselinePath,
		}
		args = append(args, extra...)
		args = append(args, filename)
		var stdout, stderr bytes.Buffer
		code := runCLIFrom(root, args, strings.NewReader(""), &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	if code, stdout, stderr := run("--generate-baseline"); code != exitSuccess || stdout != "" || stderr != "" {
		t.Fatalf("generate exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if code, stdout, stderr := run(); code != exitSuccess || withoutRunStatistics(stdout) != "0 issues\n" || stderr != "" {
		t.Fatalf("apply exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	write("package p\nfunc init() {}\nfunc init() {}\n")
	if code, stdout, stderr := run(); code != exitFindings || strings.Count(stdout, "no-init:") != 1 || stderr != "" {
		t.Fatalf("new issue exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	write("package p\n")
	if code, stdout, stderr := run(); code != exitSuccess || withoutRunStatistics(stdout) != "0 issues\n" || !strings.Contains(stderr, "1 outdated issue") {
		t.Fatalf("stale exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if code, stdout, stderr := run("--remove-outdated-baseline-entries"); code != exitSuccess || withoutRunStatistics(stdout) != "0 issues\n" || stderr != "" {
		t.Fatalf("prune exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	loaded, err := baseline.Load(baselinePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Issues) != 0 {
		t.Fatalf("pruned baseline still has issues: %#v", loaded)
	}
}

func TestSeverityFilteredBaselinePrunePreservesKnownAndRemovesUnknownCodes(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "main.go")
	baselinePath := filepath.Join(root, "strider-baseline.toml")
	if err := os.WriteFile(filename, []byte("package sample\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := baseline.Write(
		baselinePath,
		baseline.File{
			Version: baseline.Version,
			Variant: baseline.Strict,
			Issues: []baseline.Issue{
				{
					File:      "main.go",
					Code:      "no-init",
					StartLine: 1,
					EndLine:   1,
				},
				{
					File:      "main.go",
					Code:      "removed-check",
					StartLine: 1,
					EndLine:   1,
				},
			},
		},
	); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runCLI(
		[]string{
			"--no-config",
			"check",
			"--only",
			"no-init",
			"--minimum-severity",
			"error",
			"--baseline",
			baselinePath,
			"--remove-outdated-baseline-entries",
			filename,
		},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitSuccess || withoutRunStatistics(stdout.String()) != "0 issues\n" || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	loaded, err := baseline.Load(baselinePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Issues) != 1 || loaded.Issues[0].Code != "no-init" {
		t.Fatalf("catalog-aware prune wrote %#v, want only no-init", loaded.Issues)
	}
}

func TestConfiguredAnalyzerBaseline(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/baseline\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "strider.toml"), []byte("version = 1\n[check]\nbaseline = \"analysis-baseline.toml\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte("package p\nimport \"regexp\"\nvar _ = regexp.MustCompile(\"[\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runCLIFrom(root, []string{
		"check",
		"--only",
		"invalid-regexp",
		"--generate-baseline",
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("generate exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = runCLIFrom(root, []string{
		"check",
		"--only",
		"invalid-regexp",
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess || withoutRunStatistics(stdout.String()) != "0 issues\n" || stderr.Len() != 0 {
		t.Fatalf("apply exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}
