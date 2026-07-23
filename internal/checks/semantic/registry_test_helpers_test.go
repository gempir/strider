package semantic

import (
	"strings"

	"github.com/gempir/strider/internal/checks/catalog"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
)

type Registry = Plan

func (registry *Plan) Checks() []Check {
	return append([]Check(nil), registry.checks...)
}

type RegistryOptions struct {
	Only            []string
	Settings        map[string]config.CheckConfig
	Root            string
	MinimumSeverity diagnostic.Severity
}

func allChecks() []Check {
	return Catalog()
}

func NewRegistry(options RegistryOptions) (*Registry, error) {
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
	return NewPlan(selected, options.Root, options.Root != ""), nil
}

func newRegistry(only []string) (*Registry, error) {
	return NewRegistry(RegistryOptions{
		Only: only,
	})
}
