package check

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gempir/strider/internal/analyze"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/lint"
	"github.com/gempir/strider/internal/pathfilter"
)

// RegistryOptions selects and configures checks.
type RegistryOptions struct {
	Only []string
	All bool
	Settings map[string]config.RuleConfig
	MinimumSeverity diagnostic.Severity
	LintMinimumSeverity diagnostic.Severity
	AnalyzeMinimumSeverity diagnostic.Severity
	LintExcludes []string
	AnalyzeExcludes []string
	FormatExcludes []string
	Root string
}

// Registry is an immutable, capability-aware selection of checks.
type Registry struct {
	rules []Rule
	settings map[string]configuredRule
	knownCodes map[string]bool
	root string
	format bool
	lint *lint.Registry
	analyze *analyze.Registry
}

type configuredRule struct {
	severity diagnostic.Severity
	excludes []string
}

// NewRegistry builds one namespace across formatting, CST, AST, type, and SSA
// checks. Codes are case-insensitive and globally unique.
func NewRegistry(options RegistryOptions) (*Registry, error) {
	if options.All && len(options.Only) != 0 {
		return nil, fmt.Errorf("all checks and an explicit selection are mutually exclusive")
	}
	minimumSeverity, err := normalizedMinimumSeverity(options.MinimumSeverity)
	if err != nil {
		return nil, err
	}
	lintMinimumSeverity := minimumSeverity
	if options.LintMinimumSeverity != "" {
		lintMinimumSeverity, err = normalizedMinimumSeverity(options.LintMinimumSeverity)
		if err != nil {
			return nil, err
		}
	}
	analyzeMinimumSeverity := minimumSeverity
	if options.AnalyzeMinimumSeverity != "" {
		analyzeMinimumSeverity, err = normalizedMinimumSeverity(options.AnalyzeMinimumSeverity)
		if err != nil {
			return nil, err
		}
	}
	available, lintCodes, analyzeCodes, err := availableRules()
	if err != nil {
		return nil, err
	}
	normalizedSettings := make(map[string]config.RuleConfig, len(options.Settings))
	unknown := []string{}
	for code, setting := range options.Settings {
		if setting.Severity != "" && !diagnostic.ValidSeverity(
			diagnostic.Severity(setting.Severity),
		) {
			return nil, fmt.Errorf("check %q severity must be note, warning, or error", code)
		}
		normalized := strings.ToLower(code)
		if _, ok := available[normalized]; !ok {
			unknown = append(unknown, code)
			continue
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
	lintOnly := selectedCodes(wanted, lintCodes)
	analyzeOnly := selectedCodes(wanted, analyzeCodes)
	if options.All {
		// Treat --all as an explicit selection for engines whose configured
		// registry otherwise honors enabled=false. The public contract is that
		// every built-in check runs, while severity and exclusions still apply.
		analyzeOnly = selectedCodes(analyzeCodes, analyzeCodes)
	}
	lintSettings := selectedSettings(normalizedSettings, lintCodes, options.LintExcludes)
	analyzeSettings := selectedSettings(normalizedSettings, analyzeCodes, options.AnalyzeExcludes)

	registry := &Registry{
		settings: make(map[string]configuredRule, len(available)),
		knownCodes: make(map[string]bool, len(available)),
		root: options.Root,
	}
	for _, meta := range available {
		registry.knownCodes[meta.Code] = true
	}
	if !explicit || len(lintOnly) != 0 {
		registry.lint, err = lint.NewRegistryWithOptions(
			lint.RegistryOptions{
				Only: lintOnly,
				EnableAll: options.All,
				Settings: lintSettings,
				Root: options.Root,
				MinimumSeverity: lintMinimumSeverity,
			},
		)
		if err != nil {
			return nil, err
		}
	}
	if !explicit || len(analyzeOnly) != 0 {
		registry.analyze, err = analyze.NewRegistryWithOptions(
			analyze.RegistryOptions{
				Only: analyzeOnly,
				Settings: analyzeSettings,
				Root: options.Root,
				MinimumSeverity: analyzeMinimumSeverity,
			},
		)
		if err != nil {
			return nil, err
		}
	}

	formatSetting := normalizedSettings[formatMeta.Code]
	formatSetting.Excludes = append(
		append([]string(nil), options.FormatExcludes...),
		formatSetting.Excludes...,
	)
	registry.format = !explicit || wanted[formatMeta.Code]
	if formatSetting.Enabled != nil && !explicit {
		registry.format = *formatSetting.Enabled
	}
	if options.All {
		registry.format = true
	}
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

func normalizedMinimumSeverity(minimum diagnostic.Severity) (diagnostic.Severity, error) {
	if minimum == "" {
		minimum = diagnostic.SeverityNote
	}
	if !diagnostic.ValidSeverity(minimum) {
		return "", fmt.Errorf("minimum severity must be note, warning, or error")
	}
	return minimum, nil
}

func availableRules() (map[string]Meta, map[string]bool, map[string]bool, error) {
	available := map[string]Meta{formatMeta.Code: formatMeta}
	lintCodes := make(map[string]bool)
	analyzeCodes := make(map[string]bool)
	lintRegistry, err := lint.NewRegistryAll()
	if err != nil {
		return nil, nil, nil, err
	}
	for _, rule := range lintRegistry.Rules() {
		meta := rule.Meta()
		code := strings.ToLower(meta.Code)
		if _, duplicate := available[code]; duplicate {
			return nil, nil, nil, fmt.Errorf("duplicate check code %q", meta.Code)
		}
		available[code] = Meta{
			Code: meta.Code,
			Summary: meta.Summary,
			Explanation: meta.Explanation,
			GoodExample: meta.GoodExample,
			BadExample: meta.BadExample,
			DefaultSeverity: meta.DefaultSeverity,
			Capabilities: CapabilityCST,
		}
		lintCodes[code] = true
	}
	analyzeRegistry, err := analyze.NewRegistry(nil)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, rule := range analyzeRegistry.Rules() {
		meta := rule.Meta()
		code := strings.ToLower(meta.Code)
		if _, duplicate := available[code]; duplicate {
			return nil, nil, nil, fmt.Errorf("duplicate check code %q", meta.Code)
		}
		capabilities := CapabilityAST | CapabilityTypes
		requirements, requirementsOK := analyze.RequirementsFor(meta.Code)
		if requirementsOK && requirements.Facts != 0 {
			capabilities |= CapabilityFacts
		}
		if requirementsOK && requirements.Stage == analyze.AnalysisStageSSA {
			capabilities |= CapabilitySSA
		}
		available[code] = Meta{
			Code: meta.Code,
			Summary: meta.Summary,
			Explanation: meta.Explanation,
			GoodExample: meta.GoodExample,
			BadExample: meta.BadExample,
			DefaultSeverity: meta.DefaultSeverity,
			Capabilities: capabilities,
		}
		analyzeCodes[code] = true
	}
	return available, lintCodes, analyzeCodes, nil
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

func selectedSettings(
	settings map[string]config.RuleConfig,
	category map[string]bool,
	categoryExcludes []string,
) map[string]config.RuleConfig {
	result := make(map[string]config.RuleConfig, len(category))
	for code := range category {
		setting := settings[code]
		setting.Excludes = append(append([]string(nil), categoryExcludes...), setting.Excludes...)
		result[code] = setting
	}
	return result
}

func (registry *Registry) addSelectedRules(
	available map[string]Meta,
	formatSetting config.RuleConfig,
) {
	if registry.format {
		registry.rules = append(registry.rules, Rule{meta: formatMeta})
		severity := formatMeta.DefaultSeverity
		if formatSetting.Severity != "" {
			severity = diagnostic.Severity(formatSetting.Severity)
		}
		registry.settings[formatMeta.Code] = configuredRule{
			severity: severity,
			excludes: formatSetting.Excludes,
		}
	}
	if registry.lint != nil {
		for _, selected := range registry.lint.Rules() {
			meta := available[strings.ToLower(selected.Meta().Code)]
			registry.rules = append(registry.rules, Rule{meta: meta})
			registry.settings[meta.Code] = configuredRule{
				severity: registry.lint.Severity(meta.Code),
			}
		}
	}
	if registry.analyze != nil {
		for _, selected := range registry.analyze.Rules() {
			meta := available[strings.ToLower(selected.Meta().Code)]
			registry.rules = append(registry.rules, Rule{meta: meta})
			registry.settings[meta.Code] = configuredRule{
				severity: registry.analyze.Severity(meta.Code),
			}
		}
	}
	sort.Slice(
		registry.rules,
		func(left, right int) bool {
			return registry.rules[left].Meta().Code < registry.rules[right].Meta().Code
		},
	)
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
	return registry.formatApplies(filename) || registry.lint != nil && registry.lint.Applies(
		filename,
	)
}

func (registry *Registry) formatApplies(filename string) bool {
	return registry != nil && registry.format && !pathfilter.Matches(
		registry.root,
		filename,
		registry.settings[formatMeta.Code].excludes,
	)
}
