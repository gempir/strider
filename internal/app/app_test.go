package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	if err := os.WriteFile(bad, []byte("package p\nfunc F[T any](v T) T { return v }\n"), 0o600); err !=
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
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode changed to %v", info.Mode().Perm())
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
	if got := strings.Count(strings.TrimSpace(stdout.String()), "\n") + 1; got != 111 {
		t.Fatalf("listed %d rules; want 111", got)
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

func TestAnalyzeSA1000JSONAndExitCode(t *testing.T) {
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
		[]string{"analyze", "--format", "json", "--only", "sa1000"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitFindings {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"code": "SA1000"`) {
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
	if code != exitSuccess || !strings.Contains(stdout.String(), "SA1000\terror\t") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}
