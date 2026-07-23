package checks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/checks/core"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
)

func intPointer(value int) *int {
	return &value
}

func TestUnifiedRegistryHasGloballyUniqueCodes(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{})
	if err != nil {
		t.Fatal(err)
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

func TestUnifiedRegistryInventoryGolden(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, syntaxCodes, semanticCodes, err := availableChecks()
	if err != nil {
		t.Fatal(err)
	}
	var inventory strings.Builder
	for _, check := range registry.Checks() {
		meta := check.Meta()
		engine := "format"
		if syntaxCodes[meta.Code] {
			engine = "syntax"
		}
		if semanticCodes[meta.Code] {
			engine = "semantic"
		}
		fmt.Fprintf(&inventory, "%s\t%s\t%s\n", meta.Code, engine, meta.DefaultSeverity)
	}
	_, testFile, _, _ := runtime.Caller(0)
	goldenPath := filepath.Join(filepath.Dir(testFile), "testdata", "inventory.txt")
	if os.Getenv("STRIDER_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, []byte(inventory.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := inventory.String(); got != string(want) {
		t.Fatalf("unified inventory differs from testdata/inventory.txt\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestOptionSchemasAreCompleteAndRoundTrip(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range registry.Checks() {
		meta := check.Meta()
		seen := make(map[string]bool, len(meta.Options))
		for _, option := range meta.Options {
			if option.Name == "" || seen[strings.ToLower(option.Name)] {
				t.Errorf("%s has empty or duplicate option name %q", meta.Code, option.Name)
			}
			seen[strings.ToLower(option.Name)] = true
			setting := config.CheckConfig{}
			switch option.Kind {
			case core.OptionInt:
				if option.DefaultInt < 0 {
					t.Errorf("%s.%s has negative default %d", meta.Code, option.Name, option.DefaultInt)
				}
				value := option.DefaultInt + 1
				setIntOption(&setting, option.Name, &value)
				if got, ok := core.IntOption(meta, setting, option.Name); !ok || got != value {
					t.Errorf("%s.%s round-trip = %d, %t; want %d", meta.Code, option.Name, got, ok, value)
				}
			case core.OptionStrings:
				value := []string{
					"configured-value",
				}
				if option.Name == "characters" {
					value = []string{
						"_",
					}
				}
				setStringsOption(&setting, option.Name, value)
				if got, ok := core.StringsOption(meta, setting, option.Name); !ok || strings.Join(got, "\x00") != strings.Join(value, "\x00") {
					t.Errorf("%s.%s round-trip = %v, %t; want %v", meta.Code, option.Name, got, ok, value)
				}
			default:
				t.Errorf("%s.%s has unsupported kind %q", meta.Code, option.Name, option.Kind)
				continue
			}
			configured := setting.ConfiguredOptions()
			sort.Strings(configured)
			if len(configured) != 1 || configured[0] != option.Name {
				t.Errorf("%s.%s configured options = %v", meta.Code, option.Name, configured)
			}
			if err := core.ValidateOptions(meta, setting); err != nil {
				t.Errorf("%s.%s rejected its own schema value: %v", meta.Code, option.Name, err)
			}
		}
	}
}

func setIntOption(setting *config.CheckConfig, name string, value *int) {
	switch name {
	case "max-lines":
		setting.MaxLines = value
	case "max-statements":
		setting.MaxStatements = value
	case "max-results":
		setting.MaxResults = value
	case "max-parameters":
		setting.MaxParameters = value
	case "max-public-structs":
		setting.MaxPublicStructs = value
	case "max-methods":
		setting.MaxMethods = value
	}
}

func setStringsOption(setting *config.CheckConfig, name string, value []string) {
	switch name {
	case "characters":
		setting.Characters = value
	case "blocked-imports":
		setting.BlockedImports = value
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
	total := len(registry.KnownCodes())
	if got, want := len(registry.Checks()), total-len(settings); got != want {
		t.Fatalf("check count with none settings = %d, want %d", got, want)
	}
	registry, err = NewRegistry(RegistryOptions{
		Settings:        settings,
		MinimumSeverity: diagnostic.SeverityNone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(registry.Checks()), total; got != want {
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

func TestUnifiedRegistryNormalizesSettingsAndRejectsDuplicateSpellings(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"NO-INIT",
		},
		Settings: map[string]config.CheckConfig{
			"No-InIt": {
				Severity: "error",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := registry.Severity("NO-INIT"); got != diagnostic.SeverityError {
		t.Fatalf("normalized severity = %s, want error", got)
	}

	_, err = NewRegistry(RegistryOptions{
		Settings: map[string]config.CheckConfig{
			"format":  {},
			"FORMAT":  {},
			"no-init": {},
			"NO-INIT": {},
		},
	})
	if err == nil || err.Error() != `duplicate case-insensitive check setting(s): "FORMAT", "format"; "NO-INIT", "no-init"` {
		t.Fatalf("got %v, want sorted duplicate-setting error", err)
	}

	_, err = NewRegistry(RegistryOptions{
		Only: []string{
			"format",
			"FORMAT",
		},
	})
	if err == nil || !strings.Contains(err.Error(), `duplicate case-insensitive check selection(s): "FORMAT", "format"`) {
		t.Fatalf("got %v, want duplicate-selection error", err)
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

func TestUnifiedRegistryFiltersEffectiveSeverityBeforeConstruction(t *testing.T) {
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
	if registry.semantic == nil || len(registry.semantic.Checks()) != 1 || registry.semantic.Checks()[0].Meta().Code != "invalid-template" {
		t.Fatal("filtered SSA check was still constructed")
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
			MaxLines: intPointer(500),
		},
		"function-length": {
			MaxLines:      intPointer(100),
			MaxStatements: intPointer(60),
		},
		"function-result-limit": {
			MaxResults: intPointer(4),
		},
		"imports-blocklist": {
			BlockedImports: []string{
				"log",
			},
		},
		"interface-method-limit": {
			MaxMethods: intPointer(12),
		},
		"max-parameters": {
			MaxParameters: intPointer(10),
		},
		"max-public-structs": {
			MaxPublicStructs: intPointer(8),
		},
	}
	for code, setting := range tests {
		t.Run(
			code,
			func(t *testing.T) {
				if _, err := NewRegistry(RegistryOptions{
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
			MaxLines: intPointer(10),
		},
		"invalid-regexp": {
			MaxMethods: intPointer(3),
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
			MaxParameters: intPointer(4),
		},
	}
	for code, setting := range tests {
		t.Run(
			code,
			func(t *testing.T) {
				_, err := NewRegistry(RegistryOptions{
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
