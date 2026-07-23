// Package catalog provides the dependency-free contract shared by every check
// implementation. It intentionally lives below the check engines so those
// engines do not introduce an import cycle with the top-level checks package.
package catalog

import (
	"github.com/gempir/strider/internal/checkconfig"
	"github.com/gempir/strider/internal/diagnostic"
)

const (
	OptionInt     OptionKind = checkconfig.Int
	OptionStrings OptionKind = checkconfig.Strings
)

type OptionKind = checkconfig.Kind

// OptionSpec describes one configuration value supported by a check. Defaults
// live beside check metadata so selection, validation, and execution cannot
// maintain divergent option tables.
type OptionSpec struct {
	Name           string                        `json:"name"`
	Kind           OptionKind                    `json:"kind"`
	DefaultInt     int                           `json:"default_int,omitempty"`
	DefaultStrings []string                      `json:"default_strings,omitempty"`
	MinimumInt     *int                          `json:"minimum_int,omitempty"`
	MaximumInt     *int                          `json:"maximum_int,omitempty"`
	Help           string                        `json:"help"`
	Validate       func(checkconfig.Value) error `json:"-"`
}

type Option = OptionSpec

// Meta describes one user-facing check.
type Meta struct {
	Code            string              `json:"code"`
	Summary         string              `json:"summary"`
	Explanation     string              `json:"explanation"`
	GoodExample     string              `json:"good_example"`
	BadExample      string              `json:"bad_example"`
	DefaultSeverity diagnostic.Severity `json:"default_severity"`
	Options         []OptionSpec        `json:"options,omitempty"`
}

// Descriptor is the common metadata contract implemented by every check.
type Descriptor interface {
	Meta() Meta
}

type Check = Descriptor

func (meta Meta) Option(name string) (OptionSpec, bool) {
	for _, option := range meta.Options {
		if option.Name == name {
			return option, true
		}
	}
	return OptionSpec{}, false
}

// CloneMeta returns metadata with owned option/default storage.
func CloneMeta(meta Meta) Meta {
	cloned := meta
	if meta.Options != nil {
		cloned.Options = make([]OptionSpec, len(meta.Options))
		copy(cloned.Options, meta.Options)
		for index := range cloned.Options {
			cloned.Options[index].DefaultStrings = append([]string(nil), meta.Options[index].DefaultStrings...)
		}
	}
	return cloned
}

// NonNegativeIntOption declares a project-configurable non-negative integer.
func NonNegativeIntOption(name string, defaultValue int, help string) OptionSpec {
	minimum := 0
	return OptionSpec{
		Name:       name,
		Kind:       OptionInt,
		DefaultInt: defaultValue,
		MinimumInt: &minimum,
		Help:       help,
	}
}

// StringListOption declares a project-configurable list of strings.
func StringListOption(name string, defaults []string, help string) OptionSpec {
	return OptionSpec{
		Name:           name,
		Kind:           OptionStrings,
		DefaultStrings: append([]string(nil), defaults...),
		Help:           help,
	}
}
