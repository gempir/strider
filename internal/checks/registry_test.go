package checks

import (
	"strings"
	"testing"

	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
)

func TestUnifiedRegistryHasGloballyUniqueCodes(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(registry.Rules()), 227; got != want {
		t.Fatalf("all check count = %d, want %d", got, want)
	}
	seen := make(map[string]bool)
	for _, rule := range registry.Rules() {
		meta := rule.Meta()
		code := meta.Code
		if seen[code] {
			t.Fatalf("duplicate check code %q", code)
		}
		if !diagnostic.ValidSeverity(meta.DefaultSeverity) {
			t.Errorf("check %q has invalid default severity %q", code, meta.DefaultSeverity)
		}
		seen[code] = true
	}
}

func TestUnifiedRegistryPreservesDefaults(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(registry.Rules()), 118; got != want {
		t.Fatalf("default check count = %d, want %d", got, want)
	}
}

func TestUnifiedRegistryAllOverridesConfiguredDisable(t *testing.T) {
	disabled := false
	registry, err := NewRegistry(
		RegistryOptions{All: true, Settings: map[string]config.RuleConfig{"format": {Enabled: &disabled}, "no-init": {Enabled: &disabled}, "invalid-regexp": {Enabled: &disabled}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(registry.Rules()), 227; got != want {
		t.Fatalf("all check count with disabled settings = %d, want %d", got, want)
	}
}

func TestUnifiedRegistryExplicitSelectionIsCaseInsensitive(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{Only: []string{"FORMAT", "NO-INIT"}})
	if err != nil {
		t.Fatal(err)
	}
	rules := registry.Rules()
	if got, want := len(rules), 2; got != want {
		t.Fatalf("selected check count = %d, want %d", got, want)
	}
	if rules[0].Meta().Code != "format" || rules[1].Meta().Code != "no-init" {
		t.Fatalf("selected checks = %q, %q", rules[0].Meta().Code, rules[1].Meta().Code)
	}
}

func TestUnifiedRegistryCapabilitiesAvoidPackageLoadingForCSTChecks(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{Only: []string{"no-init"}})
	if err != nil {
		t.Fatal(err)
	}
	if registry.semantic != nil {
		t.Fatal("CST-only selection constructed package analyzer")
	}
	if got := registry.Rules()[0].Meta().Capabilities; got != CapabilityCST {
		t.Fatalf("no-init capabilities = %d, want CST", got)
	}
}

func TestUnifiedRegistryFiltersEffectiveSeverityBeforeCapabilities(t *testing.T) {
	registry, err := NewRegistry(
		RegistryOptions{
			Only: []string{"format", "no-init", "regexp-match-in-loop", "invalid-template"},
			Settings: map[string]config.RuleConfig{
				"format": {Severity: "warning"},
				"no-init": {Severity: "error"},
				"regexp-match-in-loop": {Severity: "warning"},
				"invalid-template": {Severity: "error"},
			},
			MinimumSeverity: diagnostic.SeverityError,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	rules := registry.Rules()
	if got := len(rules); got != 2 || rules[0].Meta().Code != "invalid-template" || rules[1].Meta().Code != "no-init" {
		t.Fatalf("filtered checks = %#v, want invalid-template and no-init", rules)
	}
	if capabilities := registry.Capabilities(); capabilities & CapabilitySSA != 0 {
		t.Fatalf("filtered SSA check still affected capabilities: %d", capabilities)
	}
	if registry.Severity("no-init") != diagnostic.SeverityError {
		t.Fatal("configured severity was not applied before filtering")
	}
}

func TestUnifiedRegistryAllDoesNotBypassMinimumSeverity(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{All: true, Settings: map[string]config.RuleConfig{"no-init": {Severity: "warning"}}, MinimumSeverity: diagnostic.SeverityError})
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range registry.Rules() {
		if rule.Meta().Code == "no-init" || rule.Meta().Code == "format" {
			t.Fatalf("--all retained below-threshold check %q", rule.Meta().Code)
		}
	}
}

func TestUnifiedRegistryRejectsInvalidMinimumSeverity(t *testing.T) {
	_, err := NewRegistry(RegistryOptions{MinimumSeverity: "fatal"})
	if err == nil {
		t.Fatal("invalid minimum severity was accepted")
	}
	_, err = NewRegistry(RegistryOptions{Only: []string{"format"}, Settings: map[string]config.RuleConfig{"no-init": {Severity: "fatal"}}})
	if err == nil || !strings.Contains(err.Error(), "severity must be") {
		t.Fatalf("got %v, want rule severity error", err)
	}
}
