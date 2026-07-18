package check

import (
	"testing"

	"github.com/gempir/strider/internal/config"
)

func TestUnifiedRegistryHasGloballyUniqueCodes(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(registry.Rules()), 203; got != want {
		t.Fatalf("all check count = %d, want %d", got, want)
	}
	seen := make(map[string]bool)
	for _, rule := range registry.Rules() {
		code := rule.Meta().Code
		if seen[code] {
			t.Fatalf("duplicate check code %q", code)
		}
		seen[code] = true
	}
}

func TestUnifiedRegistryPreservesDefaults(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(registry.Rules()), 94; got != want {
		t.Fatalf("default check count = %d, want %d", got, want)
	}
}

func TestUnifiedRegistryAllOverridesConfiguredDisable(t *testing.T) {
	disabled := false
	registry, err := NewRegistry(
		RegistryOptions{
			All: true,
			Settings: map[string]config.RuleConfig{
				"format": {Enabled: &disabled},
				"no-init": {Enabled: &disabled},
				"invalid-regexp": {Enabled: &disabled},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(registry.Rules()), 203; got != want {
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
	if registry.analyze != nil {
		t.Fatal("CST-only selection constructed package analyzer")
	}
	if got := registry.Rules()[0].Meta().Capabilities; got != CapabilityCST {
		t.Fatalf("no-init capabilities = %d, want CST", got)
	}
}
