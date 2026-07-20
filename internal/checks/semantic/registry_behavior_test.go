package semantic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
)

func TestRegistryRejectsUnknownCheck(t *testing.T) {
	if _, err := newRegistry([]string{
		"missing-analyzer",
	}); err == nil || !strings.Contains(err.Error(), "missing-analyzer") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEveryAnalyzerAcceptsCommonConfiguration(t *testing.T) {
	settings := make(map[string]config.CheckConfig, len(allChecks()))
	for _, check := range allChecks() {
		settings[check.Meta().Code] = config.CheckConfig{
			Severity: "note",
		}
	}
	registry, err := NewRegistry(RegistryOptions{
		Settings: settings,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(registry.Checks()), len(allChecks()); got != want {
		t.Fatalf("configured %d analyzers; want %d", got, want)
	}
	for _, check := range registry.Checks() {
		if severity := registry.Severity(check.Meta().Code); severity != diagnostic.SeverityNote {
			t.Errorf("%s severity = %s", check.Meta().Code, severity)
		}
	}
}

func TestAnalyzerRegistryFiltersByEffectiveSeverityBeforePlanning(t *testing.T) {
	registry, err := NewRegistry(
		RegistryOptions{
			Only: []string{
				"regexp-match-in-loop",
				"invalid-template",
			},
			Settings: map[string]config.CheckConfig{
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
	if got := len(registry.Checks()); got != 1 || registry.Checks()[0].Meta().Code != "invalid-template" {
		t.Fatalf("filtered analyzers = %#v, want invalid-template", registry.Checks())
	}
	if registry.executionPlan().needsSSA() {
		t.Fatal("filtered SSA analyzer still affected the execution plan")
	}

	registry, err = NewRegistry(
		RegistryOptions{
			Only: []string{
				"regexp-match-in-loop",
				"invalid-template",
			},
			Settings: map[string]config.CheckConfig{
				"regexp-match-in-loop": {
					Severity: "error",
				},
				"invalid-template": {
					Severity: "warning",
				},
			},
			MinimumSeverity: diagnostic.SeverityError,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(registry.Checks()); got != 1 || registry.Checks()[0].Meta().Code != "regexp-match-in-loop" {
		t.Fatalf("overridden analyzers = %#v, want regexp-match-in-loop", registry.Checks())
	}
	if !registry.executionPlan().needsSSA() {
		t.Fatal("included SSA analyzer was omitted from the execution plan")
	}
}

func TestAnalyzerRegistryRejectsInvalidMinimumSeverity(t *testing.T) {
	_, err := NewRegistry(RegistryOptions{
		MinimumSeverity: "fatal",
	})
	if err == nil || !strings.Contains(err.Error(), "minimum severity") {
		t.Fatalf("got %v, want minimum severity error", err)
	}
	_, err = NewRegistry(RegistryOptions{
		Settings: map[string]config.CheckConfig{
			"invalid-template": {
				Severity: "fatal",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "severity must be") {
		t.Fatalf("got %v, want check severity error", err)
	}
}

func TestAnalyzerRegistrySkipsLoadingWhenSeverityFilterIsEmpty(t *testing.T) {
	registry, err := NewRegistry(
		RegistryOptions{
			Only: []string{
				"suspicious-sleep",
			},
			Settings: map[string]config.CheckConfig{
				"suspicious-sleep": {
					Severity: "warning",
				},
			},
			MinimumSeverity: diagnostic.SeverityError,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("not Go source"), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatalf("empty registry attempted package loading: %v", err)
	}
	if diagnostics == nil || len(diagnostics) != 0 {
		t.Fatalf("empty registry diagnostics = %#v, want non-nil empty slice", diagnostics)
	}
}
