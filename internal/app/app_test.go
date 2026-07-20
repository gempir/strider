package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/baseline"
	"github.com/gempir/strider/internal/checks"
	"github.com/gempir/strider/internal/filewrite"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/ui"
	"github.com/gempir/strider/internal/workspace"
)

func listedSeverity(output, code string) (string, bool) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == code {
			return fields[1], true
		}
	}
	return "", false
}

func stripTerminalStyles(output string) string {
	for _, sequence := range []string{
		"\x1b[0m",
		"\x1b[1;31m",
		"\x1b[1;33m",
		"\x1b[1;34m",
	} {
		output = strings.ReplaceAll(output, sequence, "")
	}
	return output
}

func restoreWorkingDirectory(t *testing.T, directory string) {
	t.Helper()
	if err := os.Chdir(directory); err != nil {
		t.Errorf("restore working directory: %v", err)
	}
}

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"version",
	}, strings.NewReader(""), &stdout, &stderr); code != exitSuccess {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if stdout.String() != "strider dev\n" || stderr.Len() != 0 {
		t.Fatalf("stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

func TestShortOptionAliases(t *testing.T) {
	for _, arguments := range [][]string{
		{
			"-n",
			"check",
			"-s",
			"error",
			"-l",
		},
	} {
		t.Run(
			arguments[1],
			func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				if code := Run(arguments, strings.NewReader(""), &stdout, &stderr); code != exitSuccess {
					t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
				}
				if stderr.Len() != 0 {
					t.Fatalf("stdout %q, stderr %q", stdout.String(), stderr.String())
				}
			},
		)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-n",
		"fmt",
		"-s",
		"-f",
		"alias.go",
	}, strings.NewReader("package p\nfunc F( ){return}\n"), &stdout, &stderr)
	if code != exitSuccess || !strings.Contains(stdout.String(), "func F()") || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"-n",
		"check",
		"-w",
		"-f",
		"json",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitError || !strings.Contains(stderr.String(), "--watch requires text") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestLongOptionsRequireTwoDashes(t *testing.T) {
	for name, arguments := range map[string][]string{
		"global": {
			"-config",
			"missing.toml",
			"check",
		},
		"check": {
			"-n",
			"check",
			"-minimum-severity",
			"error",
		},
		"fmt": {
			"-n",
			"fmt",
			"-stdin",
		},
	} {
		t.Run(
			name,
			func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				code := Run(arguments, strings.NewReader(""), &stdout, &stderr)
				if code != exitError || !strings.Contains(stderr.String(), "must use two dashes") {
					t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
				}
				if strings.Contains(stderr.String(), "Usage:") {
					t.Fatalf("unexpected usage noise: %q", stderr.String())
				}
			},
		)
	}
}

func TestGlobalConfigFlagsConflictAfterConsumingAllArguments(t *testing.T) {
	for _, arguments := range [][]string{
		{
			"--config",
			"missing.toml",
			"--no-config",
		},
		{
			"--no-config",
			"--config=missing.toml",
		},
	} {
		var stdout, stderr bytes.Buffer
		code := Run(arguments, strings.NewReader(""), &stdout, &stderr)
		if code != exitError || !strings.Contains(stderr.String(), "--config and --no-config are mutually exclusive") {
			t.Fatalf("arguments %q: exit %d, stdout %q, stderr %q", arguments, code, stdout.String(), stderr.String())
		}
	}
}

func TestCommandUsageShowsShortAndLongOptions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-n",
		"fmt",
		"--unknown",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitError {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	for _, wanted := range []string{
		"-s, --stdin",
		"-f, --stdin-filename",
	} {
		if !strings.Contains(stderr.String(), wanted) {
			t.Fatalf("usage missing %q: %q", wanted, stderr.String())
		}
	}
	if strings.Contains(stderr.String(), "  -stdin") {
		t.Fatalf("usage contains a single-dash long option: %q", stderr.String())
	}
}

func TestCommandUsageColorsOptionsAndHidesLegacyCheckFlags(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	for _, command := range []string{
		"fmt",
		"check",
	} {
		t.Run(
			command,
			func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				code := Run([]string{
					"--color",
					"always",
					"--no-config",
					command,
					"--help",
				}, strings.NewReader(""), &stdout, &stderr)
				if code != exitError {
					t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
				}
				if !strings.Contains(stderr.String(), "\x1b[") {
					t.Fatalf("usage is not colored: %q", stderr.String())
				}
				for _, hidden := range []string{
					"--list-rules",
					"--all-rules",
				} {
					if strings.Contains(stderr.String(), hidden) {
						t.Fatalf("usage contains hidden legacy option %q: %q", hidden, stderr.String())
					}
				}
			},
		)
	}
}

func TestDoubleDashAllowsDashPrefixedPaths(t *testing.T) {
	options, ok := parseFormatOptions([]string{
		"--",
		"-generated.go",
	}, ui.ColorNever, &bytes.Buffer{})
	if !ok || len(options.paths) != 1 || options.paths[0] != "-generated.go" {
		t.Fatalf("options %#v, ok %v", options, ok)
	}
}

func TestFormatStdin(t *testing.T) {
	stdin := strings.NewReader("package p\nfunc F( ){return}\n")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"fmt",
		"--stdin",
	}, stdin, &stdout, &stderr); code != exitSuccess {
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
	t.Cleanup(func() {
		restoreWorkingDirectory(t, previous)
	})
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"fmt",
		"--check",
	}, strings.NewReader("ignored stdin"), &stdout, &stderr); code != exitFindings {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "main.go") {
		t.Fatalf("current-directory file not reported: %q", stdout.String())
	}
}

func TestFormatBatchDoesNotWriteWhenAnyFileDoesNotParse(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "good.go")
	bad := filepath.Join(root, "bad.go")
	original := []byte("package p\nfunc F( ){return}\n")
	if err := os.WriteFile(good, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("package p\nfunc F( {\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"fmt",
		"--write",
		root,
	}, strings.NewReader(""), &stdout, &stderr); code != exitError {
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

func TestFormatBatchReportsFirstFilenameError(t *testing.T) {
	root := t.TempDir()
	for name, source := range map[string]string{
		"a.go": "package p\nfunc A( {\n",
		"z.go": "package p\nfunc Z( {\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"fmt",
		"--check",
		root,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitError || !strings.Contains(stderr.String(), "a.go") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "z.go") {
		t.Fatalf("reported a later file error: %q", stderr.String())
	}
}

func TestFormatBatchReportsChangesInFilenameOrder(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"z.go",
		"a.go",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package p\nfunc F( ){return}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"fmt",
		"--check",
		root,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	a := strings.Index(stdout.String(), "a.go")
	z := strings.Index(stdout.String(), "z.go")
	if a < 0 || z < 0 || a >= z {
		t.Fatalf("changes are not filename ordered: %q", stdout.String())
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
		{
			flag: "--check",
			text: "main.go",
		},
		{
			flag: "--diff",
			text: "--- ",
		},
	} {
		assertFormatReadOnly(t, filename, original, test.flag, test.text)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"fmt",
		"--write",
		filename,
	}, strings.NewReader(""), &stdout, &stderr); code != exitSuccess {
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

func TestFormatDiffPrintsContextHunks(t *testing.T) {
	before := []byte("line-1\nline-2\nline-3\nline-4\nline-5\nline-6\nline-7\nline-8\nline-9\nline-10\n")
	after := []byte("line-1\nline-2\nline-3\nline-4\nchanged-5\nline-6\nline-7\nline-8\nline-9\nline-10\n")
	var output bytes.Buffer
	printDiff(&output, "sample.go", before, after, ui.NewPalette(&output, ui.ColorNever))
	text := output.String()
	for _, wanted := range []string{
		"@@ -2,7 +2,7 @@",
		" line-4\n",
		"-line-5\n",
		"+changed-5\n",
	} {
		if !strings.Contains(text, wanted) {
			t.Fatalf("diff missing %q:\n%s", wanted, text)
		}
	}
	for _, outsideContext := range []string{
		" line-1\n",
		" line-9\n",
		" line-10\n",
	} {
		if strings.Contains(text, outsideContext) {
			t.Fatalf("diff contains out-of-hunk context %q:\n%s", outsideContext, text)
		}
	}
}

func TestLineDiffReconstructsBothInputs(t *testing.T) {
	for name, sources := range map[string][2]string{
		"add at start": {
			"second\nthird\n",
			"first\nsecond\nthird\n",
		},
		"add at end": {
			"first\nsecond\n",
			"first\nsecond\nthird\n",
		},
		"delete middle": {
			"first\nremove\nthird\n",
			"first\nthird\n",
		},
		"replace all": {
			"old-1\nold-2\n",
			"new-1\nnew-2\n",
		},
		"line endings": {
			"first\r\nsecond\r\n",
			"first\nsecond\n",
		},
		"final newline": {
			"same",
			"same\n",
		},
		"empty to content": {
			"",
			"first\n",
		},
	} {
		t.Run(
			name,
			func(t *testing.T) {
				before := splitSourceLines([]byte(sources[0]))
				after := splitSourceLines([]byte(sources[1]))
				operations := lineDiff(before, after)
				var reconstructedBefore, reconstructedAfter []sourceLine
				for _, operation := range operations {
					if operation.kind != diffAdd {
						reconstructedBefore = append(reconstructedBefore, operation.line)
					}
					if operation.kind != diffRemove {
						reconstructedAfter = append(reconstructedAfter, operation.line)
					}
				}
				if !slices.Equal(reconstructedBefore, before) || !slices.Equal(reconstructedAfter, after) {
					t.Fatalf("operations %#v reconstruct before %#v and after %#v", operations, reconstructedBefore, reconstructedAfter)
				}
			},
		)
	}
}

func TestFormatWriterRejectsStaleSource(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "sample.go")
	original := []byte("package sample\nfunc F( ){return}\n")
	if err := os.WriteFile(filename, original, 0o640); err != nil {
		t.Fatal(err)
	}
	changed := []byte("package sample\n\nfunc changed() {}\n")
	if err := os.WriteFile(filename, changed, 0o640); err != nil {
		t.Fatal(err)
	}
	err := writeFormattedFiles(
		[]formattedFile{
			{
				filename: filename,
				original: original,
				result: formatter.Result{
					Source:  []byte("package sample\n\nfunc F() {\n\treturn\n}\n"),
					Changed: true,
				},
			},
		},
	)
	if !errors.Is(err, filewrite.ErrStale) {
		t.Fatalf("write error = %v, want ErrStale", err)
	}
	contents, readErr := os.ReadFile(filename)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(contents, changed) {
		t.Fatalf("stale write replaced current contents:\n%s", contents)
	}
}

func assertFormatReadOnly(t *testing.T, filename string, original []byte, flag, expected string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"fmt",
		flag,
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
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

func TestCheckCombinesFormattingSyntaxAndPackageChecks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/combined\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "main.go")
	original := []byte("package sample\nimport \"regexp\"\nfunc init(){regexp.MustCompile(\"[\")}\n")
	if err := os.WriteFile(filename, original, 0o600); err != nil {
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
	code := Run(
		[]string{
			"--no-config",
			"check",
			"--format",
			"json",
			"--minimum-severity",
			"note",
			"--only",
			"format,no-init,invalid-regexp",
			filename,
		},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitFindings || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	for _, wanted := range []string{
		`"code": "format"`,
		`"code": "no-init"`,
		`"code": "invalid-regexp"`,
	} {
		if !strings.Contains(stdout.String(), wanted) {
			t.Fatalf("combined report missing %q: %s", wanted, stdout.String())
		}
	}
	contents, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contents, original) {
		t.Fatal("check modified its source input")
	}
}

func TestCheckFixFormatsAndReruns(t *testing.T) {
	for _, flag := range []string{
		"--fix",
		"--fix-unsafe",
	} {
		t.Run(
			flag,
			func(t *testing.T) {
				_, filename := checkFixModule(t, "package sample\nfunc Ready( )bool{return true}\n")
				var stdout, stderr bytes.Buffer
				code := Run(
					[]string{
						"--no-config",
						"check",
						"--minimum-severity",
						"note",
						"--only",
						"format",
						flag,
						filename,
					},
					strings.NewReader(""),
					&stdout,
					&stderr,
				)
				if code != exitSuccess || stdout.String() != "0 issues\n" || stderr.Len() != 0 {
					t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
				}
				contents, err := os.ReadFile(filename)
				if err != nil {
					t.Fatal(err)
				}
				if string(contents) != "package sample\n\nfunc Ready() bool {\n\treturn true\n}\n" {
					t.Fatalf("fixed source:\n%s", contents)
				}
			},
		)
	}
}

func TestCheckFixKeepsStructuredReportOnStandardOutput(t *testing.T) {
	_, filename := checkFixModule(t, "package sample\nfunc Ready( )bool{return true}\n")
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{
			"--no-config",
			"check",
			"--format",
			"json",
			"--minimum-severity",
			"note",
			"--only",
			"format",
			"--fix",
			filename,
		},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitSuccess || stdout.String() != "[]\n" || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestCheckFixAppliesConservativeSyntaxAndSemanticFixes(t *testing.T) {
	_, filename := checkFixModule(
		t,
		`package sample

func clean(ready bool, values []int, mode int) ([]int, bool) {
	switch mode {
	case 1:
		break
	}
	return append(values), !!ready
}
`,
	)
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{
			"--no-config",
			"check",
			"--only",
			"double-negation,redundant-switch-break,single-argument-append",
			"--fix",
			filename,
		},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitSuccess || stdout.String() != "0 issues\n" || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	contents, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	want := `package sample

func clean(ready bool, values []int, mode int) ([]int, bool) {
	switch mode {
	case 1:
	}
	return (values), ready
}
`
	if string(contents) != want {
		t.Fatalf("fixed source:\n%s\nwant:\n%s", contents, want)
	}
}

func TestCheckFixReportsUnfixableFindingsAfterRerun(t *testing.T) {
	_, filename := checkFixModule(t, "package sample\nfunc init( ){return}\n")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"--no-config",
		"check",
		"--minimum-severity",
		"note",
		"--only",
		"format,no-init",
		"--fix",
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings || !strings.Contains(stdout.String(), "no-init:") || strings.Contains(stdout.String(), "format:") || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	contents, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "package sample\n\nfunc init() {\n\treturn\n}\n" {
		t.Fatalf("source was not formatted:\n%s", contents)
	}
}

func TestCheckFixHonorsFormatterExclusionsForGranularEdits(t *testing.T) {
	root, filename := checkFixModule(t, "package sample\nfunc ready(value bool)bool{return !!value}\n")
	configurationPath := filepath.Join(root, "strider.toml")
	if err := os.WriteFile(configurationPath, []byte("version = 1\n[formatter]\nexcludes = [\"sample.go\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"--config",
		configurationPath,
		"check",
		"--only",
		"double-negation",
		"--fix",
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess || stdout.String() != "0 issues\n" || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	contents, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "package sample\nfunc ready(value bool)bool{return value}\n" {
		t.Fatalf("formatter-excluded source was reformatted:\n%s", contents)
	}
}

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
	if code := Run(generate, strings.NewReader(""), &stdout, &stderr); code != exitSuccess || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("generate: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	before, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	apply := append(append([]string(nil), baseArgs...), "--fix", filename)
	if code := Run(apply, strings.NewReader(""), &stdout, &stderr); code != exitSuccess || stdout.String() != "0 issues\n" || stderr.Len() != 0 {
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

func TestCheckFixRejectsIncompatibleModes(t *testing.T) {
	for name, test := range map[string]struct {
		args []string
		want string
	}{
		"both fix levels": {
			args: []string{
				"--fix",
				"--fix-unsafe",
			},
			want: "mutually exclusive",
		},
		"watch": {
			args: []string{
				"--fix",
				"--watch",
			},
			want: "cannot be combined with --watch",
		},
		"generate": {
			args: []string{
				"--fix",
				"--generate-baseline",
			},
			want: "cannot update a baseline",
		},
		"prune": {
			args: []string{
				"--fix-unsafe",
				"--remove-outdated-baseline-entries",
			},
			want: "cannot update a baseline",
		},
	} {
		t.Run(
			name,
			func(t *testing.T) {
				args := append([]string{
					"--no-config",
					"check",
				}, test.args...)
				var stdout, stderr bytes.Buffer
				code := Run(args, strings.NewReader(""), &stdout, &stderr)
				if code != exitError || stdout.Len() != 0 || !strings.Contains(stderr.String(), test.want) {
					t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
				}
			},
		)
	}
}

func checkFixModule(t *testing.T, source string) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/checkfix\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "sample.go")
	if err := os.WriteFile(filename, []byte(source), 0o640); err != nil {
		t.Fatal(err)
	}
	return root, filename
}

func TestCheckListsOneUnifiedCatalog(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"--no-config",
		"check",
		"--minimum-severity",
		"note",
		"--list-checks",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if got := strings.Count(strings.TrimSpace(stdout.String()), "\n") + 1; got != 204 {
		t.Fatalf("listed %d checks; want 204", got)
	}
	for _, wanted := range []string{
		"format",
		"no-init",
		"invalid-regexp",
	} {
		if _, ok := listedSeverity(stdout.String(), wanted); !ok {
			t.Fatalf("unified catalog missing %q", wanted)
		}
	}
}

func TestCheckExplainsKnownCheckBelowSeverityFloor(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"--no-config",
		"check",
		"--explain",
		"NO-INIT",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	for _, wanted := range []string{
		"no-init",
		"Good:",
		"Bad:",
		"init functions hide ordering",
	} {
		if !strings.Contains(stdout.String(), wanted) {
			t.Fatalf("explanation missing %q: %q", wanted, stdout.String())
		}
	}
}

func TestCheckListAlignsAndColorsChecksBySeverity(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{
			"--color",
			"always",
			"--no-config",
			"check",
			"--minimum-severity",
			"note",
			"--only",
			"address-nil-comparison,format,invalid-regexp",
			"--list-checks",
		},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	for _, wanted := range []string{
		"\x1b[1;33maddress-nil-comparison",
		"\x1b[1;33mformat",
		"\x1b[1;31minvalid-regexp",
	} {
		if !strings.Contains(stdout.String(), wanted) {
			t.Fatalf("severity color missing %q: %q", wanted, stdout.String())
		}
	}
	plain := strings.TrimSpace(stripTerminalStyles(stdout.String()))
	severityColumn := -1
	for _, line := range strings.Split(plain, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			t.Fatalf("malformed list row %q", line)
		}
		column := strings.Index(line, fields[1])
		if severityColumn == -1 {
			severityColumn = column
		} else if column != severityColumn {
			t.Fatalf("severity columns do not align:\n%s", plain)
		}
	}
}

func TestCheckSummaryOnly(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "sample.go")
	if err := os.WriteFile(filename, []byte("package sample\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"--no-config",
		"check",
		"-s",
		"note",
		"-o",
		"no-init",
		"-q",
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings || stdout.String() != "no-init  1\n1 issue: 1 note\n" || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestCheckSummaryOnlyRejectsStructuredReports(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"--no-config",
		"check",
		"--summary-only",
		"--format",
		"json",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitError || !strings.Contains(stderr.String(), "--summary-only requires text") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestCheckDefaultsToWarningMinimumSeverity(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"--no-config",
		"check",
		"--only",
		"no-init,invalid-regexp",
		"--list-checks",
	}, strings.NewReader(""), &stdout, &stderr)
	invalidSeverity, invalidListed := listedSeverity(stdout.String(), "invalid-regexp")
	_, noInitListed := listedSeverity(stdout.String(), "no-init")
	if code != exitSuccess || stderr.Len() != 0 || noInitListed || !invalidListed || invalidSeverity != "error" {
		t.Fatalf("default severity floor: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestCheckWatchRejectsStructuredReports(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"--no-config",
		"check",
		"--watch",
		"--format",
		"json",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitError || !strings.Contains(stderr.String(), "--watch requires text") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestCheckWatcherReportsOnlyChangedGenerations(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "sample.go")
	if err := os.WriteFile(filename, []byte("package sample\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := checks.NewRegistry(checks.RegistryOptions{
		Only: []string{
			"no-init",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := checks.NewSession(registry, checks.RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	watcher := &checkWatcher{
		paths: []string{
			filename,
		},
		workspaceOptions: workspace.Options{},
		cache:            workspace.NewCache(workspace.CacheOptions{}),
		session:          session,
		colorMode:        ui.ColorNever,
		stdout:           &stdout,
		stderr:           &stderr,
	}
	if err := watcher.run(); err != nil {
		t.Fatal(err)
	}
	firstOutput := stdout.String()
	if !strings.Contains(firstOutput, "strider check #1") || !strings.Contains(firstOutput, "no-init") {
		t.Fatalf("initial output = %q", firstOutput)
	}
	if err := watcher.run(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != firstOutput {
		t.Fatalf("unchanged generation emitted output: %q", stdout.String())
	}

	if err := os.WriteFile(filename, []byte("package sample\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := watcher.run(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(stdout.String(), "== strider check #"); got != 2 {
		t.Fatalf("generation headers = %d: %q", got, stdout.String())
	}
	if !strings.Contains(stdout.String(), "strider check #2") || !strings.Contains(stdout.String(), "No findings.") {
		t.Fatalf("changed output = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestVersionOneCheckConfigurationControlsUnifiedRegistry(t *testing.T) {
	root := t.TempDir()
	configurationPath := filepath.Join(root, "strider.toml")
	configuration := `version = 1
[checks.format]
severity = "none"
[checks.no-init]
severity = "error"
[checks.invalid-regexp]
severity = "none"
`
	if err := os.WriteFile(configurationPath, []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"--config",
		configurationPath,
		"check",
		"--list-checks",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	_, formatListed := listedSeverity(stdout.String(), "format")
	_, regexpListed := listedSeverity(stdout.String(), "invalid-regexp")
	if formatListed || regexpListed {
		t.Fatalf("none-severity checks are still listed: %s", stdout.String())
	}
	if severity, ok := listedSeverity(stdout.String(), "no-init"); !ok || severity != "error" {
		t.Fatalf("configured severity is missing: %s", stdout.String())
	}
	stdout.Reset()
	code = Run([]string{
		"--config",
		configurationPath,
		"check",
		"--minimum-severity",
		"none",
		"--list-checks",
	}, strings.NewReader(""), &stdout, &stderr)
	formatSeverity, formatListed := listedSeverity(stdout.String(), "format")
	regexpSeverity, regexpListed := listedSeverity(stdout.String(), "invalid-regexp")
	if code != exitSuccess || !formatListed || formatSeverity != "none" || !regexpListed || regexpSeverity != "none" {
		t.Fatalf("none threshold did not restore suppressed checks: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestVersionOneCheckMinimumSeverityUsesEffectiveOverrides(t *testing.T) {
	root := t.TempDir()
	configurationPath := filepath.Join(root, "strider.toml")
	configuration := `version = 1
[check]
minimum-severity = "error"
[checks.no-init]
severity = "error"
[checks.suspicious-sleep]
severity = "warning"
`
	if err := os.WriteFile(configurationPath, []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"--config",
		configurationPath,
		"check",
		"--only",
		"no-init,suspicious-sleep",
		"--list-checks",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	noInitSeverity, noInitListed := listedSeverity(stdout.String(), "no-init")
	_, sleepListed := listedSeverity(stdout.String(), "suspicious-sleep")
	if !noInitListed || noInitSeverity != "error" || sleepListed {
		t.Fatalf("canonical minimum severity selected the wrong checks: %q", stdout.String())
	}

	stdout.Reset()
	code = Run(
		[]string{
			"--config",
			configurationPath,
			"check",
			"--minimum-severity",
			"warning",
			"--only",
			"no-init,suspicious-sleep",
			"--list-checks",
		},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	noInitSeverity, noInitListed = listedSeverity(stdout.String(), "no-init")
	sleepSeverity, sleepListed := listedSeverity(stdout.String(), "suspicious-sleep")
	if code != exitSuccess || !noInitListed || noInitSeverity != "error" || !sleepListed || sleepSeverity != "warning" {
		t.Fatalf("CLI minimum override: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestMinimumSeverityNoneExecutesSuppressedCheck(t *testing.T) {
	root := t.TempDir()
	configurationPath := filepath.Join(root, "strider.toml")
	if err := os.WriteFile(configurationPath, []byte("version = 1\n[checks.no-init]\nseverity = \"none\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte("package sample\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"--config",
		configurationPath,
		"check",
		"--only",
		"no-init",
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess || stdout.String() != "0 issues\n" || stderr.Len() != 0 {
		t.Fatalf("ordinary run: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	code = Run([]string{
		"--config",
		configurationPath,
		"check",
		"--minimum-severity",
		"none",
		"--only",
		"no-init",
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings || !strings.Contains(stdout.String(), "no-init:") || !strings.Contains(stdout.String(), "1 none") || stderr.Len() != 0 {
		t.Fatalf("none run: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestCategoryCommandsFilterUnifiedCheckSettings(t *testing.T) {
	root := t.TempDir()
	configurationPath := filepath.Join(root, "strider.toml")
	configuration := `version = 1
[checks.format]
severity = "none"
[checks.no-init]
severity = "error"
[checks.invalid-regexp]
severity = "warning"
`
	if err := os.WriteFile(configurationPath, []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		command          string
		includedCode     string
		includedSeverity string
		excludedCode     string
	}{} {
		t.Run(
			name,
			func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				code := Run([]string{
					"--config",
					configurationPath,
					test.command,
					"--list-rules",
				}, strings.NewReader(""), &stdout, &stderr)
				if code != exitSuccess || stderr.Len() != 0 {
					t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
				}
				severity, included := listedSeverity(stdout.String(), test.includedCode)
				_, excluded := listedSeverity(stdout.String(), test.excludedCode)
				if !included || severity != test.includedSeverity || excluded {
					t.Fatalf("canonical settings were not scoped: %q", stdout.String())
				}
			},
		)
	}
}

func TestCategoryCommandsRejectUnknownUnifiedChecks(t *testing.T) {
	root := t.TempDir()
	configurationPath := filepath.Join(root, "strider.toml")
	configuration := `version = 1
[checks.removed-check]
severity = "none"
`
	if err := os.WriteFile(configurationPath, []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{} {
		t.Run(
			command,
			func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				code := Run([]string{
					"--config",
					configurationPath,
					command,
					"--list-rules",
				}, strings.NewReader(""), &stdout, &stderr)
				if code != exitError || !strings.Contains(stderr.String(), "unknown check(s): removed-check") {
					t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
				}
			},
		)
	}
}

func TestMinimumSeverityFlagAppliesToCategoryCommands(t *testing.T) {
	root := t.TempDir()
	configurationPath := filepath.Join(root, "strider.toml")
	configuration := `version = 1
[checks.no-init]
severity = "warning"
[checks.suspicious-sleep]
severity = "warning"
`
	if err := os.WriteFile(configurationPath, []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, command := range map[string][]string{} {
		t.Run(
			name,
			func(t *testing.T) {
				args := []string{
					"--config",
					configurationPath,
				}
				args = append(args, command[0], "--minimum-severity", "error")
				args = append(args, command[1:]...)
				var stdout, stderr bytes.Buffer
				if code := Run(args, strings.NewReader(""), &stdout, &stderr); code != exitSuccess || stdout.Len() != 0 || stderr.Len() != 0 {
					t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
				}
			},
		)
	}
}

func TestCommandsRejectInvalidMinimumSeverity(t *testing.T) {
	for name, args := range map[string][]string{
		"check": {
			"check",
			"--minimum-severity",
			"fatal",
			"--list-checks",
		},
	} {
		t.Run(
			name,
			func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				arguments := append([]string{
					"--no-config",
				}, args...)
				code := Run(arguments, strings.NewReader(""), &stdout, &stderr)
				if code != exitError || !strings.Contains(stderr.String(), "--minimum-severity must be") {
					t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
				}
			},
		)
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
	code := Run(
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

func TestLintJSONAndExitCode(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte("package p\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{
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
	code := Run([]string{
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

func TestColorFlagRendersRichDiagnosticsAndLeavesJSONPlain(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	root := t.TempDir()
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte("package p\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"--color",
		"always",
		"check",
		"--minimum-severity",
		"note",
		"--only",
		"no-init",
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	for _, wanted := range []string{
		"\x1b[",
		"func \x1b[1;34minit\x1b[0m() {}",
		"┌─",
		"1 issue",
	} {
		if !strings.Contains(stdout.String(), wanted) {
			t.Fatalf("rich output missing %q: %q", wanted, stdout.String())
		}
	}

	stdout.Reset()
	code = Run([]string{
		"--color=always",
		"check",
		"--format",
		"json",
		"--minimum-severity",
		"note",
		"--only",
		"no-init",
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings || strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("JSON output should remain unstyled: exit %d, stdout %q", code, stdout.String())
	}
}

func TestConfiguredColorAndCLIOverride(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	root := t.TempDir()
	configuration := "version = 1\ncolor = \"always\"\n[check]\nminimum-severity = \"note\"\n"
	if err := os.WriteFile(filepath.Join(root, "strider.toml"), []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte("package p\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"--config",
		filepath.Join(root, "strider.toml"),
		"check",
		"--only",
		"no-init",
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings || !strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("configured color not applied: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"--config",
		filepath.Join(root, "strider.toml"),
		"--color",
		"never",
		"check",
		"--only",
		"no-init",
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings || strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("CLI color override not applied: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
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
	code := Run([]string{
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
	code := Run([]string{
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

func TestAllFlagsAreRemoved(t *testing.T) {
	for _, args := range [][]string{
		{
			"check",
			"-a",
		},
		{
			"check",
			"--all",
		},
		{
			"check",
			"--all-rules",
		},
		{
			"check",
			"-v",
			"strict",
		},
		{
			"check",
			"--baseline-variant",
			"strict",
		},
		{
			"check",
			"--baseline-variant",
			"strict",
		},
		{
			"check",
			"-B",
		},
		{
			"check",
			"--backup-baseline",
		},
		{
			"check",
			"-i",
		},
		{
			"check",
			"--ignore-baseline",
		},
		{
			"check",
			"--backup-baseline",
		},
		{
			"check",
			"--ignore-baseline",
		},
	} {
		var stdout, stderr bytes.Buffer
		code := Run(args, strings.NewReader(""), &stdout, &stderr)
		if code != exitError || !strings.Contains(stderr.String(), "flag provided but not defined") {
			t.Fatalf("args %v: exit %d, stdout %q, stderr %q", args, code, stdout.String(), stderr.String())
		}
	}
}

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

func TestProjectConfigurationControlsFormatterLintAndAnalyzer(t *testing.T) {
	root := t.TempDir()
	configuration := `version = 1
[checks.no-init]
severity = "error"
[checks.invalid-regexp]
severity = "none"
`
	if err := os.WriteFile(filepath.Join(root, "strider.toml"), []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/configured\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte("package p\nimport \"regexp\"\nfunc init() { regexp.MustCompile(\"[\") }\n"), 0o600); err != nil {
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
		"--only",
		"no-init",
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitFindings || !strings.Contains(stdout.String(), "no-init:") {
		t.Fatalf("lint exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"check",
		"--only",
		"invalid-regexp",
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess || stdout.String() != "0 issues\n" {
		t.Fatalf("analyze exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"fmt",
		"--stdin",
	}, strings.NewReader("package p\nfunc f( ){return}\n"), &stdout, &stderr)
	if code != exitSuccess || strings.Contains(stdout.String(), "\r\n") {
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
	t.Cleanup(func() {
		restoreWorkingDirectory(t, previous)
	})

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
		code := Run(args, strings.NewReader(""), &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	if code, stdout, stderr := run("--generate-baseline"); code != exitSuccess || stdout != "" || stderr != "" {
		t.Fatalf("generate exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if code, stdout, stderr := run(); code != exitSuccess || stdout != "0 issues\n" || stderr != "" {
		t.Fatalf("apply exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	write("package p\nfunc init() {}\nfunc init() {}\n")
	if code, stdout, stderr := run(); code != exitFindings || strings.Count(stdout, "no-init:") != 1 || stderr != "" {
		t.Fatalf("new issue exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	write("package p\n")
	if code, stdout, stderr := run(); code != exitSuccess || stdout != "0 issues\n" || !strings.Contains(stderr, "1 outdated issue") {
		t.Fatalf("stale exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if code, stdout, stderr := run("--remove-outdated-baseline-entries"); code != exitSuccess || stdout != "0 issues\n" || stderr != "" {
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
	code := Run(
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
	if code != exitSuccess || stdout.String() != "0 issues\n" || stderr.Len() != 0 {
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
	code = Run([]string{
		"check",
		"--only",
		"invalid-regexp",
		filename,
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess || stdout.String() != "0 issues\n" || stderr.Len() != 0 {
		t.Fatalf("apply exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}
