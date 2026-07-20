// Package core provides the dependency-free contract shared by every check
// implementation. It intentionally lives below the check engines so those
// engines do not introduce an import cycle with the top-level checks package.
package core

import "github.com/gempir/strider/internal/diagnostic"

// Meta describes one user-facing check.
type Meta struct {
	Code            string              `json:"code"`
	Summary         string              `json:"summary"`
	Explanation     string              `json:"explanation"`
	GoodExample     string              `json:"good_example"`
	BadExample      string              `json:"bad_example"`
	DefaultSeverity diagnostic.Severity `json:"default_severity"`
	Capabilities    uint8               `json:"capabilities"`
}

// Check is the common metadata contract implemented by every check.
type Check interface {
	Meta() Meta
}
