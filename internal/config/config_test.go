package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDiscoversAndAppliesDefaults(t *testing.T) {
	if got := Defaults().Formatter.MaxEmptyLines; got != 1 {
		t.Fatalf("default max empty lines = %d, want 1", got)
	}
	defaults := Defaults()
	for name, severity := range map[string]string{
		"checks": defaults.Checks.MinimumSeverity,
		"linter": defaults.Linter.MinimumSeverity,
		"analyzer": defaults.Analyzer.MinimumSeverity,
	} {
		if severity != "note" {
			t.Fatalf("default %s minimum severity = %q, want note", name, severity)
		}
	}
	root := t.TempDir()
	child := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := `version = 1
[formatter]
print-width = 120
max-empty-lines = 3
[linter.rules.no-init]
enabled = false
[analyzer.rules.invalid-regexp]
severity = "note"
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
	if configuration.Color != "auto" {
		t.Fatalf("unexpected default color mode %q", configuration.Color)
	}
	if enabled := configuration.Linter.Rules["no-init"].Enabled; enabled == nil || *enabled {
		t.Fatalf("unexpected rule config: %#v", configuration.Linter.Rules["no-init"])
	}
	if len(configuration.Checks.Excludes) != 0 || len(configuration.Checks.Rules) != 2 {
		t.Fatalf("unexpected merged checks config: %#v", configuration.Checks)
	}
	if enabled := configuration.Checks.Rules["no-init"].Enabled; enabled == nil || *enabled {
		t.Fatalf("unexpected merged check config: %#v", configuration.Checks.Rules["no-init"])
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := configuration.Resolve("lint-baseline.toml"); got != filepath.Join(
		canonicalRoot,
		"lint-baseline.toml",
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
		"version": {"version = 3\n", "unsupported configuration version"},
		"width": {"version = 1\n[formatter]\nprint-width = 20\n", "print-width"},
		"empty": {"version = 1\n[formatter]\nmax-empty-lines = -1\n", "max-empty-lines"},
		"severity": {"version = 1\n[linter.rules.no-init]\nseverity = \"fatal\"\n", "severity"},
		"checks-severity": {
			"version = 2\n[checks.rules.no-init]\nseverity = \"fatal\"\n",
			"checks.rules.no-init.severity",
		},
		"minimum-severity": {
			"version = 2\n[checks]\nminimum-severity = \"fatal\"\n",
			"checks.minimum-severity",
		},
		"legacy-minimum-severity": {
			"version = 1\n[linter]\nminimum-severity = \"fatal\"\n",
			"linter.minimum-severity",
		},
		"checks-unknown": {"version = 2\n[checks]\nunknown = true\n", "unknown configuration key"},
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

func TestLoadRejectsOverlappingLegacyRuleSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), Filename)
	contents := `version = 1
[linter.rules.shared]
severity = "warning"
[analyzer.rules.shared]
severity = "error"
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path, false)
	if err == nil || !strings.Contains(err.Error(), "configured in both legacy") {
		t.Fatalf("got %v, want overlapping legacy rule error", err)
	}
}

func TestLoadVersionTwoChecks(t *testing.T) {
	path := filepath.Join(t.TempDir(), Filename)
	contents := `version = 2
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
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration, err := Load(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Version != 2 || configuration.Checks.Baseline != "strider-baseline.toml" || configuration.Checks.BaselineVariant != "strict" || configuration.Checks.MinimumSeverity != "warning" {
		t.Fatalf("unexpected checks config: %#v", configuration.Checks)
	}
	rule := configuration.EffectiveCheckRule(LegacyLintScope, "no-init")
	if rule.Enabled == nil || *rule.Enabled || rule.Severity != "error" {
		t.Fatalf("unexpected effective rule: %#v", rule)
	}
	if strings.Join(rule.Excludes, ",") != "generated/**,legacy/**" {
		t.Fatalf("effective excludes = %q", rule.Excludes)
	}
	analysisRule := configuration.EffectiveCheckRule(LegacyAnalyzeScope, "no-init")
	if strings.Join(analysisRule.Excludes, ",") != "generated/**,legacy/**" {
		t.Fatalf("version 2 scope changed effective rule: %#v", analysisRule)
	}
}

func TestLoadVersionOnePreservesLegacyCheckScopes(t *testing.T) {
	path := filepath.Join(t.TempDir(), Filename)
	contents := `version = 1
[linter]
excludes = ["lint-only/**"]
baseline = "lint-baseline.toml"
minimum-severity = "error"
[linter.rules.no-init]
severity = "note"
excludes = ["lint-rule/**"]
[analyzer]
excludes = ["analyze-only/**"]
baseline = "analysis-baseline.toml"
baseline-variant = "strict"
minimum-severity = "warning"
[analyzer.rules.invalid-regexp]
severity = "error"
excludes = ["analysis-rule/**"]
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration, err := Load(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(configuration.Checks.Excludes) != 0 {
		t.Fatalf(
			"legacy tool exclusions were incorrectly unioned: %q",
			configuration.Checks.Excludes,
		)
	}
	if len(configuration.Checks.Rules) != 2 || configuration.Checks.Rules["no-init"].Severity != "note" || configuration.Checks.Rules["invalid-regexp"].Severity != "error" {
		t.Fatalf("unexpected merged legacy rules: %#v", configuration.Checks.Rules)
	}
	lintTool := configuration.EffectiveChecks(LegacyLintScope)
	analysisTool := configuration.EffectiveChecks(LegacyAnalyzeScope)
	if lintTool.Baseline != "lint-baseline.toml" || lintTool.MinimumSeverity != "error" || strings.Join(
		lintTool.Excludes,
		",",
	) != "lint-only/**" {
		t.Fatalf("unexpected effective lint checks: %#v", lintTool)
	}
	if analysisTool.Baseline != "analysis-baseline.toml" || analysisTool.BaselineVariant != "strict" || analysisTool.MinimumSeverity != "warning" || strings.Join(
		analysisTool.Excludes,
		",",
	) != "analyze-only/**" {
		t.Fatalf("unexpected effective analysis checks: %#v", analysisTool)
	}
	lintRule := configuration.EffectiveCheckRule(LegacyLintScope, "no-init")
	if strings.Join(lintRule.Excludes, ",") != "lint-only/**,lint-rule/**" {
		t.Fatalf("unexpected lint rule excludes: %q", lintRule.Excludes)
	}
	analysisRule := configuration.EffectiveCheckRule(LegacyAnalyzeScope, "invalid-regexp")
	if strings.Join(analysisRule.Excludes, ",") != "analyze-only/**,analysis-rule/**" {
		t.Fatalf("unexpected analysis rule excludes: %q", analysisRule.Excludes)
	}
	defaultLintRule := configuration.EffectiveCheckRule(LegacyLintScope, "unconfigured-default")
	if strings.Join(defaultLintRule.Excludes, ",") != "lint-only/**" {
		t.Fatalf("default lint rule lost tool excludes: %q", defaultLintRule.Excludes)
	}
}

func TestLoadRejectsSectionsFromAnotherConfigurationVersion(t *testing.T) {
	for name, test := range map[string]struct {
		contents string
		wanted string
	}{
		"version-one-checks": {"version = 1\n[checks]\n", "checks is only supported"},
		"implicit-version-one-checks": {"[checks]\n", "checks is only supported"},
		"version-two-linter": {"version = 2\n[linter]\n", "legacy section(s): linter"},
		"version-two-analyzer": {"version = 2\n[analyzer]\n", "legacy section(s): analyzer"},
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
	t.Cleanup(func() {
		_ = os.Chdir(previous)
	})
}
