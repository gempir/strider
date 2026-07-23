// Package checkconfig defines neutral, typed check settings shared by TOML
// decoding and catalog selection.
package checkconfig

import (
	"fmt"
	"sort"
	"strings"
)

const (
	Int     Kind = "int"
	Strings Kind = "strings"
)

type Kind string

// Value is one decoded behavioral option value.
type Value struct {
	kind    Kind
	integer int
	strings []string
}

func IntValue(value int) Value {
	return Value{
		kind:    Int,
		integer: value,
	}
}

func StringsValue(value []string) Value {
	return Value{
		kind:    Strings,
		strings: cloneStrings(value),
	}
}

func (value Value) Kind() Kind {
	return value.kind
}

func (value Value) Int() (int, bool) {
	return value.integer, value.kind == Int
}

func (value Value) Strings() ([]string, bool) {
	return cloneStrings(value.strings), value.kind == Strings
}

// Setting keeps common policy explicit and behavioral options generic.
type Setting struct {
	Severity string
	Excludes []string
	Options  map[string]Value
}

func (setting Setting) ConfiguredOptions() []string {
	names := make([]string, 0, len(setting.Options))
	for name := range setting.Options {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (setting Setting) Clone() Setting {
	cloned := setting
	cloned.Excludes = cloneStrings(setting.Excludes)
	if setting.Options != nil {
		cloned.Options = make(map[string]Value, len(setting.Options))
		for name, value := range setting.Options {
			if stringsValue, ok := value.Strings(); ok {
				cloned.Options[name] = StringsValue(stringsValue)
			} else {
				cloned.Options[name] = value
			}
		}
	}
	return cloned
}

func NormalizeCode(code string) string {
	return strings.ToLower(code)
}

func NormalizeSettings(settings map[string]Setting) (map[string]Setting, error) {
	keys := make([]string, 0, len(settings))
	for code := range settings {
		keys = append(keys, code)
	}
	sort.Strings(keys)
	normalized := make(map[string]Setting, len(settings))
	spellings := make(map[string][]string, len(settings))
	for _, code := range keys {
		canonical := NormalizeCode(code)
		spellings[canonical] = append(spellings[canonical], code)
		setting := settings[code].Clone()
		options, err := NormalizeOptions(setting.Options)
		if err != nil {
			return nil, fmt.Errorf("check %q: %w", code, err)
		}
		setting.Options = options
		normalized[canonical] = setting
	}
	if duplicates := duplicateSpellings(spellings); len(duplicates) != 0 {
		return nil, fmt.Errorf("duplicate case-insensitive check setting(s): %s", strings.Join(duplicates, "; "))
	}
	return normalized, nil
}

func NormalizeCodes(codes []string) ([]string, error) {
	normalized := make([]string, 0, len(codes))
	spellings := make(map[string][]string, len(codes))
	for _, code := range codes {
		canonical := NormalizeCode(code)
		normalized = append(normalized, canonical)
		spellings[canonical] = append(spellings[canonical], code)
	}
	if duplicates := duplicateSpellings(spellings); len(duplicates) != 0 {
		return nil, fmt.Errorf("duplicate case-insensitive check selection(s): %s", strings.Join(duplicates, "; "))
	}
	return normalized, nil
}

func NormalizeOptions(options map[string]Value) (map[string]Value, error) {
	if options == nil {
		return nil, nil
	}
	keys := make([]string, 0, len(options))
	for name := range options {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	normalized := make(map[string]Value, len(options))
	spellings := make(map[string][]string, len(options))
	for _, name := range keys {
		canonical := strings.ToLower(name)
		spellings[canonical] = append(spellings[canonical], name)
		normalized[canonical] = options[name]
	}
	if duplicates := duplicateSpellings(spellings); len(duplicates) != 0 {
		return nil, fmt.Errorf("duplicate case-insensitive check option(s): %s", strings.Join(duplicates, "; "))
	}
	return normalized, nil
}

func duplicateSpellings(spellings map[string][]string) []string {
	duplicates := make([]string, 0, len(spellings))
	for _, values := range spellings {
		if len(values) < 2 {
			continue
		}
		sort.Strings(values)
		quoted := make([]string, len(values))
		for index, value := range values {
			quoted[index] = fmt.Sprintf("%q", value)
		}
		duplicates = append(duplicates, strings.Join(quoted, ", "))
	}
	sort.Strings(duplicates)
	return duplicates
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}
