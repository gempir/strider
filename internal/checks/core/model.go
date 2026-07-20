// Package core provides the dependency-free contract shared by every check
// implementation. It intentionally lives below the check engines so those
// engines do not introduce an import cycle with the top-level checks package.
package core

import "github.com/gempir/strider/internal/diagnostic"

const (
	OptionInt     OptionKind = "int"
	OptionStrings OptionKind = "strings"
)

type OptionKind string

// Option describes one configuration value supported by a check. Defaults
// live beside check metadata so selection, validation, and execution cannot
// maintain divergent option tables.
type Option struct {
	Name           string     `json:"name"`
	Kind           OptionKind `json:"kind"`
	DefaultInt     int        `json:"default_int,omitempty"`
	DefaultStrings []string   `json:"default_strings,omitempty"`
}

// Meta describes one user-facing check.
type Meta struct {
	Code            string              `json:"code"`
	Summary         string              `json:"summary"`
	Explanation     string              `json:"explanation"`
	GoodExample     string              `json:"good_example"`
	BadExample      string              `json:"bad_example"`
	DefaultSeverity diagnostic.Severity `json:"default_severity"`
	Capabilities    uint8               `json:"capabilities"`
	Options         []Option            `json:"options,omitempty"`
}

// Check is the common metadata contract implemented by every check.
type Check interface {
	Meta() Meta
}

func (meta Meta) Option(name string) (Option, bool) {
	for _, option := range meta.Options {
		if option.Name == name {
			return option, true
		}
	}
	return Option{}, false
}
