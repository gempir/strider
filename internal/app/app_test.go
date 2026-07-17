package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/baseline"
)

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"version"}, strings.NewReader(""), &stdout, &stderr); code != exitSuccess {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if stdout.String() != "strider dev\n" || stderr.Len() != 0 {
		t.Fatalf("stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

func TestFormatStdin(t *testing.T) {
	stdin := strings.NewReader("package p\nfunc F( ){return}\n")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"fmt", "--stdin"}, stdin, &stdout, &stderr); code != exitSuccess {
		t.Fatalf("exit %d, stderr %s", code, stderr.String())
	}
	if want := "package p\n\nfunc F() {\n\treturn\n}\n"; stdout.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", stdout.String(), want)
	}
}

func TestFormatWithoutPathsScansCurrentDirectory(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte("package p\nfunc F( ){return}\n"), 0o600); err != nil {
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
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"fmt", "--check"}, strings.NewReader("ignored stdin"), &stdout, &stderr); code !=
		exitFindings {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "main.go") {
		t.Fatalf("current-directory file not reported: %q", stdout.String())
	}
}

func TestFormatBatchDoesNotWriteWhenAnyFileIsUnsupported(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "good.go")
	bad := filepath.Join(root, "bad.go")
	original := []byte("package p\nfunc F( ){return}\n")
	if err := os.WriteFile(good, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("package p\nfunc F() { goto done; done: return }\n"), 0o600); err !=
		nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"fmt", "--write", root}, strings.NewReader(""), &stdout, &stderr); code !=
		exitError {
		t.Fatalf("exit %d, stderr %s", code, stderr.String())
	}
	after, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Fatalf("supported file changed despite batch failure:\n%s", after)
	}
}

func TestFormatCheckDiffAndWrite(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "main.go")
	original := []byte("package p\nfunc F( ){return}\n")
	if err := os.WriteFile(filename, original, 0o640); err != nil {
		t.Fatal(err)
	}
	originalInfo, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		flag string
		text string
	}{
		{flag: "--check", text: "main.go"},
		{flag: "--diff", text: "--- "},
	} {
		assertFormatReadOnly(t, filename, original, test.flag, test.text)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"fmt", "--write", filename}, strings.NewReader(""), &stdout, &stderr); code !=
		exitSuccess {
		t.Fatalf("write: exit %d, stderr %q", code, stderr.String())
	}
	after, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != "package p\n\nfunc F() {\n\treturn\n}\n" {
		t.Fatalf("unexpected written source:\n%s", after)
	}
	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != originalInfo.Mode().Perm() {
		t.Fatalf("mode changed from %v to %v", originalInfo.Mode().Perm(), info.Mode().Perm())
	}
}

func assertFormatReadOnly(t *testing.T, filename string, original []byte, flag, expected string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"fmt", flag, filename}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings || !strings.Contains(stdout.String(), expected) {
		t.Fatalf("%s: exit %d, stdout %q, stderr %q", flag, code, stdout.String(), stderr.String())
	}
	after, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Fatalf("%s changed the source", flag)
	}
}

func TestLintJSONAndExitCode(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte("package p\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"lint", "--format", "json", "--only", "no-init", root},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
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
	code := Run(
		[]string{"lint", "--format", "html", "--only", "no-init", root},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitFindings {
		t.Fatalf("exit %d, stderr %s", code, stderr.String())
	}
	for _, wanted := range []string{"<!doctype html>", "Strider lint report", "no-init", "func <mark>init</mark>() {}"} {
		if !strings.Contains(stdout.String(), wanted) {
			t.Fatalf("HTML output missing %q: %s", wanted, stdout.String())
		}
	}
}

func TestColorFlagRendersRichDiagnosticsAndLeavesJSONPlain(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	root := t.TempDir()
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte("package p\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"--color", "always", "lint", "--only", "no-init", filename},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitFindings || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	for _, wanted := range []string{"\x1b[", "func init() {}", "┌─", "found 1 issue"} {
		if !strings.Contains(stdout.String(), wanted) {
			t.Fatalf("rich output missing %q: %q", wanted, stdout.String())
		}
	}

	stdout.Reset()
	code = Run(
		[]string{"--color=always", "lint", "--format", "json", "--only", "no-init", filename},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitFindings || strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("JSON output should remain unstyled: exit %d, stdout %q", code, stdout.String())
	}
}

func TestConfiguredColorAndCLIOverride(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	root := t.TempDir()
	configuration := "version = 1\ncolor = \"always\"\n"
	if err := os.WriteFile(filepath.Join(root, "strider.toml"), []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte("package p\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"--config", filepath.Join(root, "strider.toml"), "lint", "--only", "no-init", filename},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitFindings || !strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("configured color not applied: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(
		[]string{"--config", filepath.Join(root, "strider.toml"), "--color", "never", "lint", "--only", "no-init", filename},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitFindings || strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("CLI color override not applied: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestLintWithoutPathsScansCurrentDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "main.go"),
		[]byte("package p\nfunc init() {}\n"),
		0o600,
	); err != nil {
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
	var stdout, stderr bytes.Buffer
	code := Run([]string{"lint", "--only", "no-init"}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings || !strings.Contains(stdout.String(), "no-init") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestLintAllRulesListsCompleteRegistry(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"lint", "--all-rules", "--list-rules"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitSuccess {
		t.Fatalf("exit %d, stderr %s", code, stderr.String())
	}
	if got := strings.Count(strings.TrimSpace(stdout.String()), "\n") + 1; got != 116 {
		t.Fatalf("listed %d rules; want 116", got)
	}
	if !strings.Contains(stdout.String(), "marshal-receiver\t") ||
		!strings.Contains(stdout.String(), "multiline-if-init\t") {
		t.Fatalf("complete registry is missing extended rules: %s", stdout.String())
	}
}

func TestLintAllRulesAndOnlyAreMutuallyExclusive(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"lint", "--all-rules", "--only", "atomic"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitError || !strings.Contains(stderr.String(), "mutually exclusive") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestAnalyzeInvalidRegexpJSONAndExitCode(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module example.com/analyzeapp\n\ngo 1.26\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "main.go"),
		[]byte("package sample\nimport \"regexp\"\nvar _ = regexp.MustCompile(\"[\")\n"),
		0o600,
	); err != nil {
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

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"analyze", "--format", "json", "--only", "invalid-regexp"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitFindings {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"code": "invalid-regexp"`) {
		t.Fatalf("unexpected JSON: %s", stdout.String())
	}
}

func TestAnalyzeListsRules(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"analyze", "--list-rules"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitSuccess || !strings.Contains(stdout.String(), "invalid-regexp\terror\t") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestProjectConfigurationControlsFormatterLintAndAnalyzer(t *testing.T) {
	root := t.TempDir()
	configuration := `version = 1
[formatter]
end-of-line = "crlf"
max-empty-lines = 0
[linter.rules.no-init]
severity = "error"
[analyzer.rules.invalid-regexp]
enabled = false
`
	if err := os.WriteFile(filepath.Join(root, "strider.toml"), []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/configured\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(
		filename,
		[]byte("package p\nimport \"regexp\"\nfunc init() { regexp.MustCompile(\"[\") }\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	var stdout, stderr bytes.Buffer
	code := Run([]string{"lint", "--only", "no-init", filename}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings || !strings.Contains(stdout.String(), "error[no-init]") {
		t.Fatalf("lint exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"analyze", filename}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess || stdout.Len() != 0 {
		t.Fatalf("analyze exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run(
		[]string{"fmt", "--stdin"},
		strings.NewReader("package p\nfunc f( ){return}\n"),
		&stdout,
		&stderr,
	)
	if code != exitSuccess || !strings.Contains(stdout.String(), "\r\n") ||
		strings.Contains(stdout.String(), "\r\n\r\n") {
		t.Fatalf("format exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestLintBaselineGenerateApplyIgnoreAndPrune(t *testing.T) {
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
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	run := func(extra ...string) (int, string, string) {
		t.Helper()
		args := []string{"lint", "--only", "no-init", "--baseline", baselinePath}
		args = append(args, extra...)
		args = append(args, filename)
		var stdout, stderr bytes.Buffer
		code := Run(args, strings.NewReader(""), &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	if code, stdout, stderr := run("--generate-baseline"); code != exitSuccess || stdout != "" || stderr != "" {
		t.Fatalf("generate exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if code, stdout, stderr := run(); code != exitSuccess || stdout != "" || stderr != "" {
		t.Fatalf("apply exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	write("package p\nfunc init() {}\nfunc init() {}\n")
	if code, stdout, stderr := run(); code != exitFindings ||
		strings.Count(stdout, "warning[no-init]") != 1 || stderr != "" {
		t.Fatalf("new issue exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if code, stdout, stderr := run("--ignore-baseline"); code != exitFindings ||
		strings.Count(stdout, "warning[no-init]") != 2 || stderr != "" {
		t.Fatalf("ignore exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	write("package p\n")
	if code, stdout, stderr := run(); code != exitSuccess || stdout != "" ||
		!strings.Contains(stderr, "1 outdated issue") {
		t.Fatalf("stale exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if code, stdout, stderr := run("--remove-outdated-baseline-entries"); code != exitSuccess ||
		stdout != "" || stderr != "" {
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

func TestConfiguredAnalyzerBaseline(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module example.com/baseline\n\ngo 1.26\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "strider.toml"),
		[]byte("version = 1\n[analyzer]\nbaseline = \"analysis-baseline.toml\"\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(
		filename,
		[]byte("package p\nimport \"regexp\"\nvar _ = regexp.MustCompile(\"[\")\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"analyze", "--only", "invalid-regexp", "--generate-baseline", filename},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitSuccess || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("generate exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run(
		[]string{"analyze", "--only", "invalid-regexp", filename},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitSuccess || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("apply exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}
