package checks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gempir/strider/internal/checks/core"
	"github.com/gempir/strider/internal/checks/semantic"
	"github.com/gempir/strider/internal/checks/syntax"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/pathfilter"
)

// RegistryOptions selects and configures checks.
type RegistryOptions struct {
	Only            []string
	Settings        map[string]config.RuleConfig
	MinimumSeverity diagnostic.Severity
	FormatExcludes  []string
	Root            string
}

// Registry is an immutable, capability-aware selection of checks.
type Registry struct {
	rules      []Rule
	settings   map[string]configuredRule
	knownCodes map[string]bool
	root       string
	format     bool
	syntax     *syntax.Registry
	semantic   *semantic.Registry
}

type configuredRule struct {
	severity diagnostic.Severity
	excludes []string
}

// NewRegistry builds one namespace across formatting, CST, AST, type, and SSA
// checks. Codes are case-insensitive and globally unique.
func NewRegistry(options RegistryOptions) (*Registry, error) {
	available, syntaxCodes, semanticCodes, err := availableRules()
	if err != nil {
		return nil, err
	}
	settings := make(map[string]config.RuleConfig, len(options.Settings)+1)
	for code, setting := range options.Settings {
		settings[strings.ToLower(code)] = setting
	}
	formatSetting := settings[formatMeta.Code]
	formatSetting.Excludes = append(append([]string(nil), options.FormatExcludes...), formatSetting.Excludes...)
	if len(formatSetting.Excludes) != 0 || formatSetting.Severity != "" {
		settings[formatMeta.Code] = formatSetting
	}
	selection, err := core.Select(core.SelectionOptions[Rule]{
		Checks:          available,
		Only:            options.Only,
		Settings:        settings,
		MinimumSeverity: options.MinimumSeverity,
	})
	if err != nil {
		return nil, err
	}

	syntaxOnly := selectedRuleCodes(selection.Checks, syntaxCodes)
	semanticOnly := selectedRuleCodes(selection.Checks, semanticCodes)

	registry := &Registry{
		rules:      append([]Rule(nil), selection.Checks...),
		settings:   make(map[string]configuredRule, len(selection.Checks)),
		knownCodes: selection.KnownCodes,
		root:       options.Root,
	}
	for _, selected := range selection.Checks {
		meta := selected.Meta()
		setting := selection.Settings[strings.ToLower(meta.Code)]
		registry.settings[meta.Code] = configuredRule{
			severity: selection.Severities[meta.Code],
			excludes: append([]string(nil), setting.Excludes...),
		}
		registry.format = registry.format || meta.Code == formatMeta.Code
	}
	if len(syntaxOnly) != 0 {
		registry.syntax, err = syntax.NewRegistryWithOptions(
			syntax.RegistryOptions{
				Only:            syntaxOnly,
				Settings:        selectedSettings(selection.Settings, syntaxCodes),
				Root:            options.Root,
				MinimumSeverity: options.MinimumSeverity,
			},
		)
		if err != nil {
			return nil, err
		}
	}
	if len(semanticOnly) != 0 {
		registry.semantic, err = semantic.NewRegistry(
			semantic.RegistryOptions{
				Only:            semanticOnly,
				Settings:        selectedSettings(selection.Settings, semanticCodes),
				Root:            options.Root,
				MinimumSeverity: options.MinimumSeverity,
			},
		)
		if err != nil {
			return nil, err
		}
	}

	return registry, nil
}

func availableRules() ([]Rule, map[string]bool, map[string]bool, error) {
	available := []Rule{
		{
			meta: formatMeta,
		},
	}
	known := map[string]bool{
		formatMeta.Code: true,
	}
	syntaxCodes := make(map[string]bool)
	semanticCodes := make(map[string]bool)
	syntaxRegistry, err := syntax.NewRegistryWithOptions(syntax.RegistryOptions{})
	if err != nil {
		return nil, nil, nil, err
	}
	for _, rule := range syntaxRegistry.Rules() {
		meta := rule.Meta()
		code := strings.ToLower(meta.Code)
		if known[code] {
			return nil, nil, nil, fmt.Errorf("duplicate check code %q", meta.Code)
		}
		meta.Capabilities = CapabilityCST
		available = append(available, Rule{
			meta: meta,
		})
		known[code] = true
		syntaxCodes[code] = true
	}
	semanticRegistry, err := semantic.NewRegistry(semantic.RegistryOptions{})
	if err != nil {
		return nil, nil, nil, err
	}
	for _, rule := range semanticRegistry.Rules() {
		meta := rule.Meta()
		code := strings.ToLower(meta.Code)
		if known[code] {
			return nil, nil, nil, fmt.Errorf("duplicate check code %q", meta.Code)
		}
		capabilities := CapabilityAST | CapabilityTypes
		requirements := rule.Requirements()
		if requirements.Facts != 0 {
			capabilities |= CapabilityFacts
		}
		if requirements.Stage == semantic.AnalysisStageSSA {
			capabilities |= CapabilitySSA
		}
		meta.Capabilities = capabilities
		available = append(available, Rule{
			meta: meta,
		})
		known[code] = true
		semanticCodes[code] = true
	}
	sort.Slice(available, func(left, right int) bool {
		return available[left].Meta().Code < available[right].Meta().Code
	})
	return available, syntaxCodes, semanticCodes, nil
}

func selectedRuleCodes(selected []Rule, category map[string]bool) []string {
	result := make([]string, 0)
	for _, rule := range selected {
		code := strings.ToLower(rule.Meta().Code)
		if category[code] {
			result = append(result, code)
		}
	}
	sort.Strings(result)
	return result
}

func selectedSettings(settings map[string]config.RuleConfig, category map[string]bool) map[string]config.RuleConfig {
	result := make(map[string]config.RuleConfig, len(category))
	for code := range category {
		result[code] = settings[code]
	}
	return result
}

// Rules returns the selected checks in code order.
func (registry *Registry) Rules() []Rule {
	if registry == nil {
		return nil
	}
	return append([]Rule(nil), registry.rules...)
}

// KnownCodes returns every unified check code, including checks that are
// disabled or below the current severity threshold.
func (registry *Registry) KnownCodes() map[string]bool {
	if registry == nil {
		return nil
	}
	result := make(map[string]bool, len(registry.knownCodes))
	for code := range registry.knownCodes {
		result[code] = true
	}
	return result
}

// Capabilities returns the union required by the selected checks.
func (registry *Registry) Capabilities() Capability {
	if registry == nil {
		return 0
	}
	var capabilities Capability
	for _, rule := range registry.rules {
		capabilities |= rule.Meta().Capabilities
	}
	return capabilities
}

func (registry *Registry) Severity(code string) diagnostic.Severity {
	return registry.settings[strings.ToLower(code)].severity
}

func (registry *Registry) needsCST(filename string) bool {
	return registry.formatApplies(filename) || registry.syntax != nil && registry.syntax.Applies(filename)
}

func (registry *Registry) formatApplies(filename string) bool {
	return registry != nil && registry.format && !pathfilter.Excluded(registry.root, filename, registry.settings[formatMeta.Code].excludes)
}
