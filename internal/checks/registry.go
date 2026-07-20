package checks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gempir/strider/internal/checks/semantic"
	"github.com/gempir/strider/internal/checks/syntax"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/pathfilter"
)

var supportedBehavioralOptions = map[string]map[string]bool{
	"banned-characters": {
		"characters": true,
	},
	"file-length-limit": {
		"max-lines": true,
	},
	"function-length": {
		"max-lines":      true,
		"max-statements": true,
	},
	"function-result-limit": {
		"max-results": true,
	},
	"imports-blocklist": {
		"blocked-imports": true,
	},
	"interface-method-limit": {
		"max-methods": true,
	},
	"max-parameters": {
		"max-parameters": true,
	},
	"max-public-structs": {
		"max-public-structs": true,
	},
}

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
	minimumSeverity, err := normalizedMinimumSeverity(options.MinimumSeverity)
	if err != nil {
		return nil, err
	}
	available, syntaxCodes, semanticCodes, err := availableRules()
	if err != nil {
		return nil, err
	}
	normalizedSettings := make(map[string]config.RuleConfig, len(options.Settings))
	unknown := []string{}
	for code, setting := range options.Settings {
		if setting.Severity != "" && !diagnostic.ValidSeverity(diagnostic.Severity(setting.Severity)) {
			return nil, fmt.Errorf("check %q severity must be none, note, warning, or error", code)
		}
		normalized := strings.ToLower(code)
		if _, ok := available[normalized]; !ok {
			unknown = append(unknown, code)
			continue
		}
		if err := validateBehavioralOptions(normalized, setting); err != nil {
			return nil, err
		}
		normalizedSettings[normalized] = setting
	}
	wanted := make(map[string]bool, len(options.Only))
	for _, code := range options.Only {
		normalized := strings.ToLower(code)
		if _, ok := available[normalized]; !ok {
			unknown = append(unknown, code)
			continue
		}
		wanted[normalized] = true
	}
	if len(unknown) != 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown check(s): %s", strings.Join(unknown, ", "))
	}

	explicit := len(options.Only) != 0
	syntaxOnly := selectedCodes(wanted, syntaxCodes)
	semanticOnly := selectedCodes(wanted, semanticCodes)
	syntaxSettings := selectedSettings(normalizedSettings, syntaxCodes)
	semanticSettings := selectedSettings(normalizedSettings, semanticCodes)

	registry := &Registry{
		settings:   make(map[string]configuredRule, len(available)),
		knownCodes: make(map[string]bool, len(available)),
		root:       options.Root,
	}
	for _, meta := range available {
		registry.knownCodes[meta.Code] = true
	}
	if !explicit || len(syntaxOnly) != 0 {
		registry.syntax, err = syntax.NewRegistryWithOptions(
			syntax.RegistryOptions{
				Only:            syntaxOnly,
				Settings:        syntaxSettings,
				Root:            options.Root,
				MinimumSeverity: minimumSeverity,
			},
		)
		if err != nil {
			return nil, err
		}
	}
	if !explicit || len(semanticOnly) != 0 {
		registry.semantic, err = semantic.NewRegistry(
			semantic.RegistryOptions{
				Only:            semanticOnly,
				Settings:        semanticSettings,
				Root:            options.Root,
				MinimumSeverity: minimumSeverity,
			},
		)
		if err != nil {
			return nil, err
		}
	}

	formatSetting := normalizedSettings[formatMeta.Code]
	formatSetting.Excludes = append(append([]string(nil), options.FormatExcludes...), formatSetting.Excludes...)
	registry.format = !explicit || wanted[formatMeta.Code]
	formatSeverity := formatMeta.DefaultSeverity
	if formatSetting.Severity != "" {
		formatSeverity = diagnostic.Severity(formatSetting.Severity)
	}
	if !formatSeverity.AtLeast(minimumSeverity) {
		registry.format = false
	}
	registry.addSelectedRules(available, formatSetting)
	return registry, nil
}

func validateBehavioralOptions(code string, setting config.RuleConfig) error {
	configured := []struct {
		name    string
		present bool
	}{
		{
			"characters",
			setting.Characters != nil,
		},
		{
			"max-lines",
			setting.MaxLines != 0,
		},
		{
			"max-statements",
			setting.MaxStatements != 0,
		},
		{
			"max-results",
			setting.MaxResults != 0,
		},
		{
			"max-parameters",
			setting.MaxParameters != 0,
		},
		{
			"max-public-structs",
			setting.MaxPublicStructs != 0,
		},
		{
			"max-methods",
			setting.MaxMethods != 0,
		},
		{
			"blocked-imports",
			setting.BlockedImports != nil,
		},
	}
	for index := range configured {
		configured[index].present = configured[index].present || setting.HasExplicitOption(configured[index].name)
	}

	allowed := supportedBehavioralOptions[code]
	for _, option := range configured {
		if option.present && !allowed[option.name] {
			return fmt.Errorf("check %q does not support %s", code, option.name)
		}
	}
	return nil
}

func normalizedMinimumSeverity(minimum diagnostic.Severity) (diagnostic.Severity, error) {
	if minimum == "" {
		minimum = diagnostic.SeverityNote
	}
	if !diagnostic.ValidSeverity(minimum) {
		return "", fmt.Errorf("minimum severity must be none, note, warning, or error")
	}
	return minimum, nil
}

func availableRules() (map[string]Meta, map[string]bool, map[string]bool, error) {
	available := map[string]Meta{
		formatMeta.Code: formatMeta,
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
		if _, duplicate := available[code]; duplicate {
			return nil, nil, nil, fmt.Errorf("duplicate check code %q", meta.Code)
		}
		available[code] = Meta{
			Code:            meta.Code,
			Summary:         meta.Summary,
			Explanation:     meta.Explanation,
			GoodExample:     meta.GoodExample,
			BadExample:      meta.BadExample,
			DefaultSeverity: meta.DefaultSeverity,
			Capabilities:    CapabilityCST,
		}
		syntaxCodes[code] = true
	}
	semanticRegistry, err := semantic.NewRegistry(semantic.RegistryOptions{})
	if err != nil {
		return nil, nil, nil, err
	}
	for _, rule := range semanticRegistry.Rules() {
		meta := rule.Meta()
		code := strings.ToLower(meta.Code)
		if _, duplicate := available[code]; duplicate {
			return nil, nil, nil, fmt.Errorf("duplicate check code %q", meta.Code)
		}
		capabilities := CapabilityAST | CapabilityTypes
		requirements, requirementsOK := semantic.RequirementsFor(meta.Code)
		if requirementsOK && requirements.Facts != 0 {
			capabilities |= CapabilityFacts
		}
		if requirementsOK && requirements.Stage == semantic.AnalysisStageSSA {
			capabilities |= CapabilitySSA
		}
		available[code] = Meta{
			Code:            meta.Code,
			Summary:         meta.Summary,
			Explanation:     meta.Explanation,
			GoodExample:     meta.GoodExample,
			BadExample:      meta.BadExample,
			DefaultSeverity: meta.DefaultSeverity,
			Capabilities:    capabilities,
		}
		semanticCodes[code] = true
	}
	return available, syntaxCodes, semanticCodes, nil
}

func selectedCodes(wanted, category map[string]bool) []string {
	result := make([]string, 0)
	for code := range wanted {
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

func (registry *Registry) addSelectedRules(available map[string]Meta, formatSetting config.RuleConfig) {
	if registry.format {
		registry.rules = append(registry.rules, Rule{
			meta: formatMeta,
		})
		severity := formatMeta.DefaultSeverity
		if formatSetting.Severity != "" {
			severity = diagnostic.Severity(formatSetting.Severity)
		}
		registry.settings[formatMeta.Code] = configuredRule{
			severity: severity,
			excludes: formatSetting.Excludes,
		}
	}
	if registry.syntax != nil {
		for _, selected := range registry.syntax.Rules() {
			meta := available[strings.ToLower(selected.Meta().Code)]
			registry.rules = append(registry.rules, Rule{
				meta: meta,
			})
			registry.settings[meta.Code] = configuredRule{
				severity: registry.syntax.Severity(meta.Code),
			}
		}
	}
	if registry.semantic != nil {
		for _, selected := range registry.semantic.Rules() {
			meta := available[strings.ToLower(selected.Meta().Code)]
			registry.rules = append(registry.rules, Rule{
				meta: meta,
			})
			registry.settings[meta.Code] = configuredRule{
				severity: registry.semantic.Severity(meta.Code),
			}
		}
	}
	sort.Slice(registry.rules, func(left, right int) bool {
		return registry.rules[left].Meta().Code < registry.rules[right].Meta().Code
	})
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
	return registry != nil && registry.format && !pathfilter.Matches(registry.root, filename, registry.settings[formatMeta.Code].excludes)
}
