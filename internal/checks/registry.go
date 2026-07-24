//strider:ignore-file cognitive-complexity,confusing-naming,cyclomatic-complexity,function-length,import-shadowing,modifies-parameter,single-case-switch,use-slices-sort
package checks

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/gempir/strider/internal/checks/catalog"
	"github.com/gempir/strider/internal/checks/semantic"
	"github.com/gempir/strider/internal/checks/syntax"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/pathfilter"
	"github.com/gempir/strider/internal/source"
)

// RegistryOptions selects and configures checks.
type RegistryOptions struct {
	Only            []string
	Settings        map[string]config.CheckConfig
	MinimumSeverity diagnostic.Severity
	FormatExcludes  []string
	Root            string
	Directory       string
}

// Registry is an immutable unified selection and prepared execution plan.
type Registry struct {
	checks     []Descriptor
	settings   map[string]configuredCheck
	knownCodes map[string]bool
	root       string
	format     bool
	syntax     *syntax.Plan
	semantic   *semantic.Plan
	display    sync.Map
}

type configuredCheck struct {
	severity diagnostic.Severity
	excludes []string
}

type catalogEntry struct {
	meta     Meta
	engine   Engine
	syntax   syntax.Check
	semantic semantic.Descriptor
}

func (entry catalogEntry) Meta() Meta {
	return catalog.CloneMeta(entry.meta)
}

func (entry catalogEntry) Engine() Engine {
	return entry.engine
}

// NewRegistry builds one namespace across formatting, CST, AST, type, and SSA
// checks. Codes are case-insensitive and globally unique.
func NewRegistry(options RegistryOptions) (*Registry, error) {
	rootSet := options.Root != ""
	options.Root = source.ResolveRoot(options.Root)
	available := availableChecks()
	settings := make(map[string]config.CheckConfig, len(options.Settings)+1)
	settingCodes := make([]string, 0, len(options.Settings))
	for code, setting := range options.Settings {
		settings[code] = config.CloneCheckConfig(setting)
		settingCodes = append(settingCodes, code)
	}
	sort.Strings(settingCodes)
	formatCode := formatMeta.Code
	for _, code := range settingCodes {
		if strings.EqualFold(code, formatMeta.Code) {
			formatCode = code
			break
		}
	}
	formatSetting := settings[formatCode]
	formatSetting.Excludes = append(append([]string(nil), options.FormatExcludes...), formatSetting.Excludes...)
	if len(formatSetting.Excludes) != 0 || formatSetting.Severity != "" {
		settings[formatCode] = formatSetting
	}
	selection, err := catalog.Select(catalog.SelectionOptions[catalogEntry]{
		Checks:          available,
		Only:            options.Only,
		Settings:        settings,
		MinimumSeverity: options.MinimumSeverity,
	})
	if err != nil {
		return nil, err
	}

	registry := &Registry{
		checks:     make([]Descriptor, 0, len(selection.Checks)),
		settings:   make(map[string]configuredCheck, len(selection.Checks)),
		knownCodes: selection.KnownCodes,
		root:       options.Root,
	}
	syntaxPlan := make([]syntax.SelectedCheck, 0)
	semanticPlan := make([]semantic.SelectedCheck, 0)
	for _, selected := range selection.Checks {
		meta := selected.Meta()
		setting := selection.Settings[strings.ToLower(meta.Code)]
		registry.checks = append(registry.checks, selected)
		registry.settings[meta.Code] = configuredCheck{
			severity: selection.Severities[meta.Code],
			excludes: append([]string(nil), setting.Excludes...),
		}
		switch selected.engine {
		case EngineFormat:
			registry.format = true
		case EngineSyntax:
			syntaxPlan = append(
				syntaxPlan,
				syntax.SelectedCheck{
					Check:    selected.syntax,
					Severity: selection.Severities[meta.Code],
					Excludes: setting.Excludes,
					Options:  selection.Options[meta.Code],
				},
			)
		case EngineSemantic:
			semanticPlan = append(
				semanticPlan,
				semantic.SelectedCheck{
					Check:    selected.semantic,
					Severity: selection.Severities[meta.Code],
					Excludes: setting.Excludes,
					Options:  selection.Options[meta.Code],
				},
			)
		}
	}
	if len(syntaxPlan) != 0 {
		registry.syntax = syntax.NewPlan(syntaxPlan, options.Root)
	}
	if len(semanticPlan) != 0 {
		registry.semantic = semantic.NewPlan(semanticPlan, options.Root, rootSet, options.Directory)
	}

	return registry, nil
}

func availableChecks() []catalogEntry {
	available := []catalogEntry{
		{
			meta:   formatMeta,
			engine: EngineFormat,
		},
	}
	for _, check := range syntax.Catalog() {
		meta := check.Meta()
		available = append(available, catalogEntry{
			meta:   meta,
			engine: EngineSyntax,
			syntax: check,
		})
	}
	for _, check := range semantic.Catalog() {
		meta := check.Meta()
		available = append(available, catalogEntry{
			meta:     meta,
			engine:   EngineSemantic,
			semantic: check,
		})
	}
	sort.Slice(available, func(left, right int) bool {
		return available[left].Meta().Code < available[right].Meta().Code
	})
	return available
}

// Checks returns the selected checks in code order.
func (registry *Registry) Checks() []Descriptor {
	if registry == nil {
		return nil
	}
	return append([]Descriptor(nil), registry.checks...)
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

func (registry *Registry) Severity(code string) diagnostic.Severity {
	return registry.settings[strings.ToLower(code)].severity
}

func (registry *Registry) needsCST(filename string) bool {
	return registry.formatApplies(filename) || registry.syntax != nil && registry.syntax.Applies(filename)
}

func (registry *Registry) formatApplies(filename string) bool {
	return registry != nil && registry.format && !pathfilter.Excluded(registry.root, filename, registry.settings[formatMeta.Code].excludes)
}

func (registry *Registry) diagnosticPath(filename string) string {
	if cached, ok := registry.display.Load(filename); ok {
		if display, valid := cached.(string); valid {
			return display
		}
	}
	display := source.DiagnosticPath(registry.root, filename)
	cached, _ := registry.display.LoadOrStore(filename, display)
	if stored, valid := cached.(string); valid {
		return stored
	}
	return display
}

func (registry *Registry) localCacheIdentity(filename string) string {
	if registry == nil {
		return ""
	}
	var identity strings.Builder
	if registry.formatApplies(filename) {
		setting := registry.settings[formatMeta.Code]
		fmt.Fprintf(&identity, "format=true\nseverity=%s\n", setting.severity)
		excludes := append([]string(nil), setting.excludes...)
		sort.Strings(excludes)
		for _, exclude := range excludes {
			fmt.Fprintf(&identity, "format-exclude=%s\n", exclude)
		}
	} else {
		identity.WriteString("format=false\n")
	}
	if registry.syntax == nil {
		identity.WriteString("syntax=disabled\n")
	} else {
		identity.WriteString(registry.syntax.CacheIdentity(filename))
	}
	return identity.String()
}
