package syntax

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
)

func TestCatalogIsCompleteDocumentedAndRunnable(t *testing.T) {
	all, err := NewRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	checks := all.Checks()
	names := make([]string, 0, len(checks))
	seen := map[string]bool{}
	_, testFile, _, _ := runtime.Caller(0)
	docsDirectory := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "docs", "src", "content", "docs", "lints")
	fixture := writeFixture(t, "// Package p is a fixture.\npackage p\n")
	coreCodes := map[string]bool{
		"cyclomatic-complexity": true,
		"max-parameters":        true,
		"no-defer-in-loop":      true,
		"no-else-after-return":  true,
		"no-init":               true,
		"no-naked-return":       true,
		"no-package-var":        true,
	}
	for _, check := range checks {
		meta := check.Meta()
		if seen[meta.Code] {
			t.Errorf("duplicate check %q", meta.Code)
		}
		seen[meta.Code] = true
		names = append(names, meta.Code)
		if strings.TrimSpace(meta.GoodExample) == "" || strings.TrimSpace(meta.BadExample) == "" {
			t.Errorf("check %s has incomplete examples", meta.Code)
		}
		if strings.HasPrefix(meta.GoodExample, "See the check reference") || strings.HasPrefix(meta.BadExample, "See the check reference") {
			t.Errorf("check %s still has placeholder examples", meta.Code)
		}
		if !coreCodes[meta.Code] && (meta.Explanation == meta.Summary+"." || !strings.Contains(meta.Explanation, "Default:")) {
			t.Errorf("extended check %s explanation does not add its default contract: %q", meta.Code, meta.Explanation)
		}
		if _, err := os.Stat(filepath.Join(docsDirectory, meta.Code+".md")); err != nil {
			t.Errorf("check %s has no documentation: %v", meta.Code, err)
		}
		registry, err := NewRegistry([]string{
			meta.Code,
		})
		if err != nil {
			t.Errorf("select %s: %v", meta.Code, err)
			continue
		}
		if _, err := Run([]string{
			fixture,
		}, registry); err != nil {
			t.Errorf("run %s: %v", meta.Code, err)
		}
	}
	sort.Strings(names)
	want, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "testdata", "check_codes.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(names, "\n") + "\n"; got != string(want) {
		t.Errorf("check catalog differs from testdata/check_codes.txt\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestEveryLintCheckAcceptsCommonConfiguration(t *testing.T) {
	all, err := NewRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	settings := make(map[string]config.CheckConfig, len(all.Checks()))
	for _, check := range all.Checks() {
		settings[check.Meta().Code] = config.CheckConfig{
			Severity: "note",
		}
	}
	configured, err := NewRegistryConfigured(nil, settings, "")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(configured.Checks()), len(all.Checks()); got != want {
		t.Fatalf("configured %d checks; want %d", got, want)
	}
	for _, check := range configured.Checks() {
		if severity := configured.Severity(check.Meta().Code); severity != diagnostic.SeverityNote {
			t.Errorf("%s severity = %s", check.Meta().Code, severity)
		}
	}
}

func TestBannedCharactersUsesDefaultsAndConfiguration(t *testing.T) {
	filename := writeFixture(t, "package p\nvar ᐸname, under_score int\n")
	for name, test := range map[string]struct {
		settings map[string]config.CheckConfig
		wanted   string
	}{
		"defaults": {
			wanted: "ᐸ",
		},
		"configured": {
			settings: map[string]config.CheckConfig{
				"banned-characters": {
					Options: map[string]config.OptionValue{
						"characters": config.StringsValue([]string{
							"_",
						}),
					},
				},
			},
			wanted: "_",
		},
	} {
		t.Run(
			name,
			func(t *testing.T) {
				registry, err := NewRegistryWithOptions(RegistryOptions{
					Only: []string{
						"banned-characters",
					},
					Settings: test.settings,
				})
				if err != nil {
					t.Fatal(err)
				}
				diagnostics, err := Run([]string{
					filename,
				}, registry)
				if err != nil {
					t.Fatal(err)
				}
				assertDiagnosticGolden(t, diagnostics)
				if !strings.Contains(diagnostics[0].Message, test.wanted) {
					t.Fatalf("diagnostics = %#v, want one finding for %q", diagnostics, test.wanted)
				}
				if diagnostics[0].Severity != diagnostic.SeverityError {
					t.Fatalf("severity = %s, want error", diagnostics[0].Severity)
				}
			},
		)
	}
}

func TestBannedCharactersRejectsInvalidConfiguration(t *testing.T) {
	for name, settings := range map[string]map[string]config.CheckConfig{
		"multiple runes": {
			"banned-characters": {
				Options: map[string]config.OptionValue{
					"characters": config.StringsValue([]string{
						"ab",
					}),
				},
			},
		},
		"unrelated check": {
			"no-init": {
				Options: map[string]config.OptionValue{
					"characters": config.StringsValue([]string{
						"_",
					}),
				},
			},
		},
	} {
		t.Run(
			name,
			func(t *testing.T) {
				if _, err := NewRegistryWithOptions(RegistryOptions{
					Settings: settings,
				}); err == nil {
					t.Fatal("expected invalid character configuration to fail")
				}
			},
		)
	}
}

func TestSyntaxRegistryFiltersByEffectiveSeverityBeforeExecution(t *testing.T) {
	for name, options := range map[string]RegistryOptions{
		"only": {
			Only: []string{
				"no-init",
			},
			Settings: map[string]config.CheckConfig{
				"no-init": {
					Severity: "warning",
				},
			},
			MinimumSeverity: diagnostic.SeverityError,
		},
		"all": {
			Settings: map[string]config.CheckConfig{
				"no-init": {
					Severity: "warning",
				},
			},
			MinimumSeverity: diagnostic.SeverityError,
		},
	} {
		t.Run(
			name,
			func(t *testing.T) {
				registry, err := NewRegistryWithOptions(options)
				if err != nil {
					t.Fatal(err)
				}
				for _, check := range registry.Checks() {
					if check.Meta().Code == "no-init" {
						t.Fatal("selection bypassed the minimum severity")
					}
				}
				if name == "only" {
					diagnostics, runErr := Run([]string{
						filepath.Join(t.TempDir(), "missing.go"),
					}, registry)
					if runErr != nil {
						t.Fatalf("empty registry attempted CST execution: %v", runErr)
					}
					if diagnostics == nil || len(diagnostics) != 0 {
						t.Fatalf("empty registry diagnostics = %#v", diagnostics)
					}
				}
			},
		)
	}

	registry, err := NewRegistryWithOptions(
		RegistryOptions{
			Only: []string{
				"no-init",
			},
			Settings: map[string]config.CheckConfig{
				"no-init": {
					Severity: "error",
				},
			},
			MinimumSeverity: diagnostic.SeverityError,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(registry.Checks()); got != 1 {
		t.Fatalf("checks after severity override = %d, want 1", got)
	}
	if severity := registry.Severity("no-init"); severity != diagnostic.SeverityError {
		t.Fatalf("effective severity = %s, want error", severity)
	}
}

func TestSyntaxRegistryRejectsInvalidMinimumSeverity(t *testing.T) {
	_, err := NewRegistryWithOptions(RegistryOptions{
		MinimumSeverity: "fatal",
	})
	if err == nil || !strings.Contains(err.Error(), "minimum severity") {
		t.Fatalf("got %v, want minimum severity error", err)
	}
	_, err = NewRegistryWithOptions(RegistryOptions{
		Settings: map[string]config.CheckConfig{
			"no-init": {
				Severity: "fatal",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "severity must be") {
		t.Fatalf("got %v, want check severity error", err)
	}
}

func TestSyntaxCheckConfigurationCanExcludePaths(t *testing.T) {
	fixture := writeFixture(t, "package p\nfunc init() {}\n")
	registry, err := NewRegistryConfigured([]string{
		"no-init",
	}, map[string]config.CheckConfig{
		"no-init": {
			Excludes: []string{
				"**/*.go",
			},
		},
	}, filepath.Dir(fixture))
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		fixture,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics == nil {
		t.Fatal("clean single-file lint returned a nil diagnostics slice")
	}
	if len(diagnostics) != 0 {
		t.Fatalf("excluded check reported diagnostics: %#v", diagnostics)
	}
}
