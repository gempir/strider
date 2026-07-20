package core

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
)

// SelectionOptions controls the common check-selection policy used by every
// check engine. Codes are case-insensitive; explicit selection is still
// subject to the minimum effective severity.
type SelectionOptions[T Check] struct {
	Checks          []T
	Only            []string
	Settings        map[string]config.CheckConfig
	MinimumSeverity diagnostic.Severity
	Validate        func(code string, setting config.CheckConfig) error
}

// Selection is the normalized result of applying SelectionOptions.
type Selection[T Check] struct {
	Checks     []T
	Settings   map[string]config.CheckConfig
	Severities map[string]diagnostic.Severity
	KnownCodes map[string]bool
}

// Select normalizes configuration, rejects unknown codes, resolves effective
// severities, and applies explicit-code and minimum-severity filters.
func Select[T Check](options SelectionOptions[T]) (Selection[T], error) {
	minimumSeverity := options.MinimumSeverity
	if minimumSeverity == "" {
		minimumSeverity = diagnostic.SeverityNote
	}
	if !diagnostic.ValidSeverity(minimumSeverity) {
		return Selection[T]{}, fmt.Errorf("minimum severity must be none, note, warning, or error")
	}

	byCode := make(map[string]T, len(options.Checks))
	knownCodes := make(map[string]bool, len(options.Checks))
	for _, check := range options.Checks {
		code := strings.ToLower(check.Meta().Code)
		if _, duplicate := byCode[code]; duplicate {
			return Selection[T]{}, fmt.Errorf("duplicate check code %q", check.Meta().Code)
		}
		byCode[code] = check
		knownCodes[check.Meta().Code] = true
	}

	settings := make(map[string]config.CheckConfig, len(options.Settings))
	unknown := make([]string, 0)
	for code, setting := range options.Settings {
		if setting.Severity != "" && !diagnostic.ValidSeverity(diagnostic.Severity(setting.Severity)) {
			return Selection[T]{}, fmt.Errorf("check %q severity must be none, note, warning, or error", code)
		}
		normalized := strings.ToLower(code)
		if _, ok := byCode[normalized]; !ok {
			unknown = append(unknown, code)
			continue
		}
		if options.Validate != nil {
			if err := options.Validate(byCode[normalized].Meta().Code, setting); err != nil {
				return Selection[T]{}, err
			}
		}
		settings[normalized] = setting
	}

	wanted := make(map[string]bool, len(options.Only))
	for _, code := range options.Only {
		normalized := strings.ToLower(code)
		if _, ok := byCode[normalized]; !ok {
			unknown = append(unknown, code)
			continue
		}
		wanted[normalized] = true
	}
	if len(unknown) != 0 {
		sort.Strings(unknown)
		return Selection[T]{}, fmt.Errorf("unknown check(s): %s", strings.Join(unknown, ", "))
	}

	selection := Selection[T]{
		Checks:     make([]T, 0, len(options.Checks)),
		Settings:   settings,
		Severities: make(map[string]diagnostic.Severity, len(options.Checks)),
		KnownCodes: knownCodes,
	}
	for _, check := range options.Checks {
		meta := check.Meta()
		code := strings.ToLower(meta.Code)
		if len(wanted) != 0 && !wanted[code] {
			continue
		}
		severity := meta.DefaultSeverity
		if setting := settings[code]; setting.Severity != "" {
			severity = diagnostic.Severity(setting.Severity)
		}
		if !severity.AtLeast(minimumSeverity) {
			continue
		}
		selection.Checks = append(selection.Checks, check)
		selection.Severities[meta.Code] = severity
	}
	return selection, nil
}
