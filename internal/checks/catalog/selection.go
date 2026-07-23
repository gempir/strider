package catalog

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gempir/strider/internal/checkconfig"
	"github.com/gempir/strider/internal/diagnostic"
)

// SelectionOptions controls the common check-selection policy used by every
// check engine. Codes are case-insensitive; explicit selection is still
// subject to the minimum effective severity.
type SelectionOptions[T Descriptor] struct {
	Checks          []T
	Only            []string
	Settings        map[string]checkconfig.Setting
	MinimumSeverity diagnostic.Severity
}

// Selection is the normalized result of applying SelectionOptions.
type Selection[T Descriptor] struct {
	Checks     []T
	Settings   map[string]checkconfig.Setting
	Severities map[string]diagnostic.Severity
	Options    map[string]ResolvedOptions
	KnownCodes map[string]bool
}

// ResolvedOptions is an immutable, schema-bound option set. It contains
// configured values where present and descriptor defaults otherwise.
type ResolvedOptions struct {
	values map[string]checkconfig.Value
}

// Int returns a bound integer option.
func (options ResolvedOptions) Int(name string) (int, bool) {
	value, ok := options.values[name]
	if !ok {
		return 0, false
	}
	return value.Int()
}

// Strings returns a defensive copy of a bound string-list option.
func (options ResolvedOptions) Strings(name string) ([]string, bool) {
	value, ok := options.values[name]
	if !ok {
		return nil, false
	}
	return value.Strings()
}

// Select normalizes configuration, rejects unknown codes, resolves effective
// severities, and applies explicit-code and minimum-severity filters.
func Select[T Descriptor](options SelectionOptions[T]) (Selection[T], error) {
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
		meta := check.Meta()
		if err := ValidateSchema(meta); err != nil {
			return Selection[T]{}, err
		}
		code := strings.ToLower(meta.Code)
		if _, duplicate := byCode[code]; duplicate {
			return Selection[T]{}, fmt.Errorf("duplicate check code %q", meta.Code)
		}
		byCode[code] = check
		knownCodes[meta.Code] = true
	}

	settings, err := checkconfig.NormalizeSettings(options.Settings)
	if err != nil {
		return Selection[T]{}, err
	}
	resolvedByCode := make(map[string]ResolvedOptions, len(options.Checks))
	unknown := make([]string, 0)
	settingCodes := make([]string, 0, len(settings))
	for code := range settings {
		settingCodes = append(settingCodes, code)
	}
	sort.Strings(settingCodes)
	for _, code := range settingCodes {
		setting := settings[code].Clone()
		if setting.Severity != "" && !diagnostic.ValidSeverity(diagnostic.Severity(setting.Severity)) {
			return Selection[T]{}, fmt.Errorf("check %q severity must be none, note, warning, or error", code)
		}
		if _, ok := byCode[code]; !ok {
			unknown = append(unknown, code)
			continue
		}
		resolved, err := ResolveOptions(byCode[code].Meta(), setting)
		if err != nil {
			return Selection[T]{}, err
		}
		resolvedByCode[code] = resolved
		settings[code] = setting
	}

	only, err := checkconfig.NormalizeCodes(options.Only)
	if err != nil {
		return Selection[T]{}, err
	}
	wanted := make(map[string]bool, len(only))
	for _, code := range only {
		if _, ok := byCode[code]; !ok {
			unknown = append(unknown, code)
			continue
		}
		wanted[code] = true
	}
	if len(unknown) != 0 {
		sort.Strings(unknown)
		return Selection[T]{}, fmt.Errorf("unknown check(s): %s", strings.Join(unknown, ", "))
	}

	selection := Selection[T]{
		Checks:     make([]T, 0, len(options.Checks)),
		Settings:   settings,
		Severities: make(map[string]diagnostic.Severity, len(options.Checks)),
		Options:    make(map[string]ResolvedOptions, len(options.Checks)),
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
		resolved, ok := resolvedByCode[code]
		if !ok {
			var err error
			resolved, err = ResolveOptions(meta, checkconfig.Setting{})
			if err != nil {
				return Selection[T]{}, err
			}
		}
		selection.Checks = append(selection.Checks, check)
		selection.Severities[meta.Code] = severity
		selection.Options[meta.Code] = resolved
	}
	return selection, nil
}

// ValidateSchema checks descriptor-owned option facts, including defaults.
func ValidateSchema(meta Meta) error {
	if meta.Code == "" {
		return fmt.Errorf("check code must not be empty")
	}
	if !diagnostic.ValidSeverity(meta.DefaultSeverity) {
		return fmt.Errorf("check %q has invalid default severity %q", meta.Code, meta.DefaultSeverity)
	}
	seen := make(map[string]bool, len(meta.Options))
	for _, spec := range meta.Options {
		if spec.Name == "" || spec.Name != strings.ToLower(spec.Name) {
			return fmt.Errorf("check %q has invalid option name %q", meta.Code, spec.Name)
		}
		if seen[spec.Name] {
			return fmt.Errorf("check %q has duplicate option name %q", meta.Code, spec.Name)
		}
		seen[spec.Name] = true
		if strings.TrimSpace(spec.Help) == "" {
			return fmt.Errorf("check %q option %s has no help text", meta.Code, spec.Name)
		}
		var value checkconfig.Value
		switch spec.Kind {
		case OptionInt:
			value = checkconfig.IntValue(spec.DefaultInt)
		case OptionStrings:
			value = checkconfig.StringsValue(spec.DefaultStrings)
		default:
			return fmt.Errorf("check %q option %s has unsupported kind %q", meta.Code, spec.Name, spec.Kind)
		}
		if err := validateOptionValue(meta.Code, spec, value); err != nil {
			return err
		}
	}
	return nil
}

// ValidateOptions rejects behavioral settings not declared by meta.
func ValidateOptions(meta Meta, setting checkconfig.Setting) error {
	for _, name := range setting.ConfiguredOptions() {
		spec, supported := meta.Option(name)
		if !supported {
			return fmt.Errorf("check %q does not support %s", meta.Code, name)
		}
		value := setting.Options[name]
		if err := validateOptionValue(meta.Code, spec, value); err != nil {
			return err
		}
	}
	return nil
}

func validateOptionValue(code string, spec OptionSpec, value checkconfig.Value) error {
	if value.Kind() != spec.Kind {
		return fmt.Errorf("check %q option %s must be %s", code, spec.Name, spec.Kind)
	}
	if integer, ok := value.Int(); ok {
		if spec.MinimumInt != nil && integer < *spec.MinimumInt {
			return fmt.Errorf("check %q option %s must be at least %d", code, spec.Name, *spec.MinimumInt)
		}
		if spec.MaximumInt != nil && integer > *spec.MaximumInt {
			return fmt.Errorf("check %q option %s must be at most %d", code, spec.Name, *spec.MaximumInt)
		}
	}
	if spec.Validate != nil {
		if err := spec.Validate(value); err != nil {
			return fmt.Errorf("check %q option %s: %w", code, spec.Name, err)
		}
	}
	return nil
}

// ResolveOptions validates and binds a setting to a descriptor schema.
func ResolveOptions(meta Meta, setting checkconfig.Setting) (ResolvedOptions, error) {
	if err := ValidateOptions(meta, setting); err != nil {
		return ResolvedOptions{}, err
	}
	values := make(map[string]checkconfig.Value, len(meta.Options))
	for _, spec := range meta.Options {
		value, configured := setting.Options[spec.Name]
		if !configured {
			switch spec.Kind {
			case OptionInt:
				value = checkconfig.IntValue(spec.DefaultInt)
			case OptionStrings:
				value = checkconfig.StringsValue(spec.DefaultStrings)
			}
			if err := validateOptionValue(meta.Code, spec, value); err != nil {
				return ResolvedOptions{}, err
			}
		}
		values[spec.Name] = value
	}
	return ResolvedOptions{
		values: values,
	}, nil
}
