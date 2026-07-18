package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultsUseVersionOneAndWideFormatting(t *testing.T) {
	defaults := Defaults()
	if defaults.Version != 1 {
		t.Fatalf("default version = %d, want 1", defaults.Version)
	}
	if defaults.Formatter.PrintWidth != 180 || defaults.Formatter.MaxEmptyLines != 1 {
		t.Fatalf("unexpected formatter defaults: %#v", defaults.Formatter)
	}
	if defaults.Checks.MinimumSeverity != "warning" {
		t.Fatalf("default minimum severity = %q, want warning", defaults.Checks.MinimumSeverity)
	}
}

func TestLoadDiscoversVersionOneChecks(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := `version = 1
[formatter]
print-width = 120
max-empty-lines = 3
[checks]
excludes = ["generated/**"]
baseline = "strider-baseline.toml"
baseline-variant = "strict"
minimum-severity = "warning"
[checks.rules.no-init]
enabled = false
severity = "error"
excludes = ["legacy/**"]
`
	if err := os.WriteFile(filepath.Join(root, Filename), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	chdir(t, child)
	configuration, err := Load("", false)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Formatter.PrintWidth != 120 || configuration.Formatter.IndentWidth != 4 || configuration.Formatter.MaxEmptyLines != 3 {
		t.Fatalf("unexpected formatter config: %#v", configuration.Formatter)
	}
	if configuration.Checks.Baseline != "strider-baseline.toml" || configuration.Checks.BaselineVariant != "strict" || configuration.Checks.MinimumSeverity != "warning" {
		t.Fatalf("unexpected checks config: %#v", configuration.Checks)
	}
	rule := configuration.EffectiveCheckRule("no-init")
	if rule.Enabled == nil || *rule.Enabled || rule.Severity != "error" {
		t.Fatalf("unexpected effective rule: %#v", rule)
	}
	if strings.Join(rule.Excludes, ",") != "generated/**,legacy/**" {
		t.Fatalf("effective excludes = %q", rule.Excludes)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := configuration.Resolve("strider-baseline.toml"); got != filepath.Join(
		canonicalRoot,
		"strider-baseline.toml",
	) {
		t.Fatalf("resolved path %q", got)
	}
}

func TestLoadRejectsUnknownAndInvalidSettings(t *testing.T) {
	for name, test := range map[string]struct {
		contents string
		wanted string
	}{
		"unknown": {"version = 1\nunknown = true\n", "unknown configuration key"},
		"version": {"version = 9\n", "expected 1"},
		"width": {"version = 1\n[formatter]\nprint-width = 20\n", "print-width"},
		"empty": {"version = 1\n[formatter]\nmax-empty-lines = -1\n", "max-empty-lines"},
		"severity": {"version = 1\n[checks.rules.no-init]\nseverity = \"fatal\"\n", "severity"},
		"minimum-severity": {
			"version = 1\n[checks]\nminimum-severity = \"fatal\"\n",
			"minimum-severity",
		},
		"checks-unknown": {"version = 1\n[checks]\nunknown = true\n", "unknown configuration key"},
		"legacy-linter": {"version = 1\n[linter]\n", "unknown configuration key"},
		"legacy-analyzer": {"version = 1\n[analyzer]\n", "unknown configuration key"},
		"color": {"version = 1\ncolor = \"sometimes\"\n", "color"},
	} {
		t.Run(
			name,
			func(t *testing.T) {
				path := filepath.Join(t.TempDir(), Filename)
				if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
					t.Fatal(err)
				}
				_,
				err := Load(path, false)
				if err == nil || !strings.Contains(err.Error(), test.wanted) {
					t.Fatalf("got %v, want error containing %q", err, test.wanted)
				}
			},
		)
	}
}

func chdir(t *testing.T, directory string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(
		func() {
			if err := os.Chdir(previous); err != nil {
				t.Errorf("restore working directory: %v", err)
			}
		},
	)
}
