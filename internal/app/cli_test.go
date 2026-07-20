package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/ui"
)

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
