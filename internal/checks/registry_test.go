package checks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
)

func TestUnifiedRegistryHasGloballyUniqueCodes(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(registry.Checks()), 207; got != want {
		t.Fatalf("all check count = %d, want %d", got, want)
	}
	descriptiveCode := regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)+$`)
	seen := make(map[string]bool)
	for _, check := range registry.Checks() {
		meta := check.Meta()
		code := meta.Code
		if seen[code] {
			t.Fatalf("duplicate check code %q", code)
		}
		if !diagnostic.ValidSeverity(meta.DefaultSeverity) {
			t.Errorf("check %q has invalid default severity %q", code, meta.DefaultSeverity)
		}
		if code != "format" && !descriptiveCode.MatchString(code) {
			t.Errorf("check %q is too generic; use a descriptive kebab-case code", code)
		}
		if strings.TrimSpace(meta.Summary) == "" || strings.TrimSpace(meta.Explanation) == "" {
			t.Errorf("check %q has incomplete prose metadata", code)
		}
		if strings.EqualFold(strings.TrimSuffix(strings.TrimSpace(meta.Explanation), "."), strings.TrimSuffix(strings.TrimSpace(meta.Summary), ".")) {
			t.Errorf("check %q explanation only repeats its summary", code)
		}
		if strings.TrimSpace(meta.GoodExample) == "" || strings.TrimSpace(meta.BadExample) == "" || meta.GoodExample == meta.BadExample {
			t.Errorf("check %q has incomplete or identical examples", code)
		}
		seen[code] = true
	}
}

func TestUnifiedRegistrySelectsEveryCheckByDefault(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(registry.Checks()), 207; got != want {
		t.Fatalf("check count = %d, want %d", got, want)
	}
}

func TestUnifiedRegistryNoneSeverityDisablesUnlessRequested(t *testing.T) {
	settings := map[string]config.CheckConfig{
		"format": {
			Severity: "none",
		},
		"no-init": {
			Severity: "none",
		},
		"invalid-regexp": {
			Severity: "none",
		},
	}
	registry, err := NewRegistry(RegistryOptions{
		Settings: settings,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(registry.Checks()), 204; got != want {
		t.Fatalf("check count with none settings = %d, want %d", got, want)
	}
	registry, err = NewRegistry(RegistryOptions{
		Settings:        settings,
		MinimumSeverity: diagnostic.SeverityNone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(registry.Checks()), 207; got != want {
		t.Fatalf("none-threshold check count = %d, want %d", got, want)
	}
	for code := range settings {
		if severity := registry.Severity(code); severity != diagnostic.SeverityNone {
			t.Errorf("%s severity = %s, want none", code, severity)
		}
	}
}

func TestUnifiedRegistryPolicySeverities(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{
		MinimumSeverity: diagnostic.SeverityNote,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]diagnostic.Severity{
		"blank-imports":                      diagnostic.SeverityWarning,
		"boolean-literal-comparison":         diagnostic.SeverityWarning,
		"confusing-naming":                   diagnostic.SeverityError,
		"confusing-results":                  diagnostic.SeverityWarning,
		"context-as-argument":                diagnostic.SeverityWarning,
		"cyclomatic-complexity":              diagnostic.SeverityWarning,
		"deep-exit":                          diagnostic.SeverityError,
		"dot-imports":                        diagnostic.SeverityWarning,
		"double-negation":                    diagnostic.SeverityError,
		"duplicated-imports":                 diagnostic.SeverityError,
		"early-return":                       diagnostic.SeverityWarning,
		"redundant-atomic-result-assignment": diagnostic.SeverityError,
	}
	for code, severity := range want {
		if got := registry.Severity(code); got != severity {
			t.Errorf("%s severity = %s, want %s", code, got, severity)
		}
	}
	for _, removed := range []string{
		"comment-spacings",
		"cyclomatic",
	} {
		if severity := registry.Severity(removed); severity != "" {
			t.Errorf("removed check %s still has severity %s", removed, severity)
		}
	}
}

func TestUnifiedRegistryExplicitSelectionIsCaseInsensitive(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"FORMAT",
			"NO-INIT",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	checks := registry.Checks()
	if got, want := len(checks), 2; got != want {
		t.Fatalf("selected check count = %d, want %d", got, want)
	}
	if checks[0].Meta().Code != "format" || checks[1].Meta().Code != "no-init" {
		t.Fatalf("selected checks = %q, %q", checks[0].Meta().Code, checks[1].Meta().Code)
	}
}

func TestUnifiedRegistryCapabilitiesAvoidPackageLoadingForCSTChecks(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"no-init",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if registry.semantic != nil {
		t.Fatal("CST-only selection constructed package analyzer")
	}
	if got := registry.Checks()[0].Meta().Capabilities; got != CapabilityCST {
		t.Fatalf("no-init capabilities = %d, want CST", got)
	}
}

func TestUnifiedRegistryFiltersEffectiveSeverityBeforeCapabilities(t *testing.T) {
	registry, err := NewRegistry(
		RegistryOptions{
			Only: []string{
				"format",
				"no-init",
				"regexp-match-in-loop",
				"invalid-template",
			},
			Settings: map[string]config.CheckConfig{
				"format": {
					Severity: "warning",
				},
				"no-init": {
					Severity: "error",
				},
				"regexp-match-in-loop": {
					Severity: "warning",
				},
				"invalid-template": {
					Severity: "error",
				},
			},
			MinimumSeverity: diagnostic.SeverityError,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	checks := registry.Checks()
	if got := len(checks); got != 2 || checks[0].Meta().Code != "invalid-template" || checks[1].Meta().Code != "no-init" {
		t.Fatalf("filtered checks = %#v, want invalid-template and no-init", checks)
	}
	if capabilities := registry.Capabilities(); capabilities&CapabilitySSA != 0 {
		t.Fatalf("filtered SSA check still affected capabilities: %d", capabilities)
	}
	if registry.Severity("no-init") != diagnostic.SeverityError {
		t.Fatal("configured severity was not applied before filtering")
	}
}

func TestUnifiedRegistrySelectionDoesNotBypassMinimumSeverity(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{
		Settings: map[string]config.CheckConfig{
			"no-init": {
				Severity: "warning",
			},
		},
		MinimumSeverity: diagnostic.SeverityError,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range registry.Checks() {
		if check.Meta().Code == "no-init" || check.Meta().Code == "format" {
			t.Fatalf("registry retained below-threshold check %q", check.Meta().Code)
		}
	}
}

func TestUnifiedRegistryRejectsInvalidMinimumSeverity(t *testing.T) {
	_, err := NewRegistry(RegistryOptions{
		MinimumSeverity: "fatal",
	})
	if err == nil {
		t.Fatal("invalid minimum severity was accepted")
	}
	_, err = NewRegistry(RegistryOptions{
		Only: []string{
			"format",
		},
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

func TestUnifiedRegistryAcceptsOnlySupportedBehavioralOptions(t *testing.T) {
	tests := map[string]config.CheckConfig{
		"banned-characters": {
			Characters: []string{
				"_",
			},
		},
		"file-length-limit": {
			MaxLines: 500,
		},
		"function-length": {
			MaxLines:      100,
			MaxStatements: 60,
		},
		"function-result-limit": {
			MaxResults: 4,
		},
		"imports-blocklist": {
			BlockedImports: []string{
				"log",
			},
		},
		"interface-method-limit": {
			MaxMethods: 12,
		},
		"max-parameters": {
			MaxParameters: 10,
		},
		"max-public-structs": {
			MaxPublicStructs: 8,
		},
	}
	for code, setting := range tests {
		t.Run(
			code,
			func(t *testing.T) {
				if _,
					err := NewRegistry(RegistryOptions{
					Settings: map[string]config.CheckConfig{
						code: setting,
					},
				}); err != nil {
					t.Fatalf("supported option was rejected: %v", err)
				}
			},
		)
	}
}

func TestUnifiedRegistryRejectsBehavioralOptionOnWrongCheck(t *testing.T) {
	tests := map[string]config.CheckConfig{
		"no-init": {
			MaxLines: 10,
		},
		"invalid-regexp": {
			MaxMethods: 3,
		},
		"format": {
			BlockedImports: []string{},
		},
		"function-length": {
			Characters: []string{
				"_",
			},
		},
		"interface-method-limit": {
			MaxParameters: 4,
		},
	}
	for code, setting := range tests {
		t.Run(
			code,
			func(t *testing.T) {
				_,
					err := NewRegistry(RegistryOptions{
					Settings: map[string]config.CheckConfig{
						code: setting,
					},
				})
				if err == nil || !strings.Contains(err.Error(), "does not support") {
					t.Fatalf("got %v, want unsupported-option error", err)
				}
			},
		)
	}
}

func TestUnifiedRegistryRejectsExplicitZeroOptionOnWrongCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.Filename)
	contents := "version = 1\n[checks.no-init]\nmax-lines = 0\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration, err := config.Load(path, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewRegistry(RegistryOptions{
		Settings: configuration.Checks.Settings,
	})
	if err == nil || !strings.Contains(err.Error(), "does not support max-lines") {
		t.Fatalf("got %v, want unsupported max-lines error", err)
	}
}
