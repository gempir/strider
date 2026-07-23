package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/checks"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/ui"
	"github.com/gempir/strider/internal/workspace"
)

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
	var stdout, stderr bytes.Buffer
	code := runCLIFrom(
		root,
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
				code := runCLI(
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
	code := runCLI(
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
	code := runCLI(
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
	code := runCLI([]string{
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
	code := runCLI([]string{
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
		"no package loading": {
			args: []string{
				"--fix",
				"--no-package-loading",
			},
			want: "requires package loading",
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
				code := runCLI(args, strings.NewReader(""), &stdout, &stderr)
				if code != exitError || stdout.Len() != 0 || !strings.Contains(stderr.String(), test.want) {
					t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
				}
			},
		)
	}
}

func TestCheckCanSkipPackageLoading(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte("package sample\nimport \"regexp\"\nfunc init() { regexp.MustCompile(\"[\") }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runCLIFrom(
		root,
		[]string{
			"--no-config",
			"check",
			"--no-package-loading",
			"--format",
			"json",
			"--minimum-severity",
			"note",
			"--only",
			"no-init,invalid-regexp",
			filename,
		},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitFindings || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"code": "no-init"`) {
		t.Fatalf("syntax finding missing: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), `"code": "invalid-regexp"`) {
		t.Fatalf("package-aware finding was not skipped: %s", stdout.String())
	}
}

func TestCheckConfigurationCanSkipPackageLoading(t *testing.T) {
	root := t.TempDir()
	configurationPath := filepath.Join(root, config.Filename)
	if err := os.WriteFile(configurationPath, []byte("version = 1\n[check]\npackage-loading = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte("package sample\nimport \"regexp\"\nfunc init() { regexp.MustCompile(\"[\") }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runCLI(
		[]string{
			"--config",
			configurationPath,
			"check",
			"--format",
			"json",
			"--minimum-severity",
			"note",
			"--only",
			"no-init,invalid-regexp",
			filename,
		},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if code != exitFindings || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), `"code": "invalid-regexp"`) {
		t.Fatalf("package-aware finding was not skipped: %s", stdout.String())
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
	code := runCLI([]string{
		"--no-config",
		"check",
		"--minimum-severity",
		"note",
		"--list-checks",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	registry, err := checks.NewRegistry(checks.RegistryOptions{
		MinimumSeverity: "note",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Count(strings.TrimSpace(stdout.String()), "\n")+1, len(registry.Checks()); got != want {
		t.Fatalf("listed %d checks; want %d", got, want)
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
	code := runCLI([]string{
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
	code := runCLI(
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
	code := runCLI([]string{
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
	code := runCLI([]string{
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
	code := runCLI([]string{
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
	code := runCLI([]string{
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
	var stdout, stderr bytes.Buffer
	watcher := &checkWatcher{
		paths: []string{
			filename,
		},
		workspaceOptions: workspace.Options{},
		cache:            workspace.NewCache(workspace.CacheOptions{}),
		registry:         registry,
		runOptions:       checks.RunOptions{},
		colorMode:        ui.ColorNever,
		stdout:           &stdout,
		stderr:           &stderr,
	}
	if err := watcher.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	firstOutput := stdout.String()
	if !strings.Contains(firstOutput, "strider check #1") || !strings.Contains(firstOutput, "no-init") {
		t.Fatalf("initial output = %q", firstOutput)
	}
	if err := watcher.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != firstOutput {
		t.Fatalf("unchanged generation emitted output: %q", stdout.String())
	}

	if err := os.WriteFile(filename, []byte("package sample\nfunc init() {} // changed source, same finding\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := watcher.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != firstOutput {
		t.Fatalf("diagnostically unchanged source emitted output: %q", stdout.String())
	}

	if err := os.WriteFile(filename, []byte("package sample\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := watcher.run(context.Background()); err != nil {
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
	code := runCLI([]string{
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
	code = runCLI([]string{
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
	code := runCLI([]string{
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
	code = runCLI(
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
	code := runCLI([]string{
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
	code = runCLI([]string{
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
				code := runCLI([]string{
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
				code := runCLI([]string{
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
				if code := runCLI(args, strings.NewReader(""), &stdout, &stderr); code != exitSuccess || stdout.Len() != 0 || stderr.Len() != 0 {
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
				code := runCLI(arguments, strings.NewReader(""), &stdout, &stderr)
				if code != exitError || !strings.Contains(stderr.String(), "--minimum-severity must be") {
					t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
				}
			},
		)
	}
}
