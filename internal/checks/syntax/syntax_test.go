//strider:ignore-file cognitive-complexity,top-level-declaration-order
package syntax

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/checks/catalog"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/diagnostic"
)

type Registry = Plan

func (r *Plan) Checks() []Check {
	return append([]Check(nil), r.checks...)
}

func NewRegistry(only []string) (*Registry, error) {
	return NewRegistryConfigured(only, nil, "")
}

func NewRegistryConfigured(only []string, settings map[string]config.CheckConfig, root string) (*Registry, error) {
	return NewRegistryWithOptions(RegistryOptions{
		Only:            only,
		Settings:        settings,
		Root:            root,
		MinimumSeverity: diagnostic.SeverityNote,
	})
}

type RegistryOptions struct {
	Only            []string
	Settings        map[string]config.CheckConfig
	Root            string
	MinimumSeverity diagnostic.Severity
}

func NewRegistryWithOptions(options RegistryOptions) (*Registry, error) {
	selection, err := catalog.Select(catalog.SelectionOptions[Check]{
		Checks:          Catalog(),
		Only:            options.Only,
		Settings:        options.Settings,
		MinimumSeverity: options.MinimumSeverity,
	})
	if err != nil {
		return nil, err
	}
	selected := make([]SelectedCheck, 0, len(selection.Checks))
	for _, check := range selection.Checks {
		meta := check.Meta()
		setting := selection.Settings[strings.ToLower(meta.Code)]
		selected = append(selected, SelectedCheck{
			Check:    check,
			Severity: selection.Severities[meta.Code],
			Excludes: setting.Excludes,
			Options:  selection.Options[meta.Code],
		})
	}
	return NewPlan(selected, options.Root), nil
}

// Run is a test-only adapter over the production AnalyzeTree boundary.
func Run(files []string, registry *Registry) ([]diagnostic.Diagnostic, error) {
	diagnostics := []diagnostic.Diagnostic{}
	for _, filename := range files {
		if !registry.Applies(filename) {
			continue
		}
		contents, err := os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		tree, err := cst.Parse(filename, contents)
		if err != nil {
			return nil, err
		}
		diagnostics = append(diagnostics, AnalyzeTree(filename, tree, registry)...)
	}
	diagnostic.Sort(diagnostics)
	return diagnostics, nil
}

func writeFixture(t *testing.T, source string) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "fixture.go")
	if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return filename
}

func BenchmarkLint(b *testing.B) {
	filename := filepath.Join(b.TempDir(), "fixture.go")
	if err := os.WriteFile(filename, []byte("package p\nfunc F(a int) int { if a > 0 { return a }; return -a }\n"), 0o600); err != nil {
		b.Fatal(err)
	}
	registry, err := NewRegistry(nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for range b.N {
		if _, err := Run([]string{
			filename,
		}, registry); err != nil {
			b.Fatal(err)
		}
	}
}
