// Package checks implements Strider's unified diagnostic pipeline.
package checks

import (
	"github.com/gempir/strider/internal/checks/core"
	"github.com/gempir/strider/internal/diagnostic"
)

const (
	CapabilitySource Capability = 1 << iota
	CapabilityCST
	CapabilityAST
	CapabilityTypes
	CapabilityFacts
	CapabilitySSA
)

var formatMeta = Meta{
	Code:            "format",
	Summary:         "require canonical Strider formatting",
	Explanation:     "Canonical formatting keeps Go source deterministic and removes style-only review noise.",
	GoodExample:     "package main\n\nfunc main() {}",
	BadExample:      "package main\nfunc main( ){ }",
	DefaultSeverity: diagnostic.SeverityWarning,
	Capabilities:    CapabilityCST,
}

// Capability describes the most expensive program representation required by
// a check. Capabilities are internal scheduling details, not CLI categories.
type Capability = uint8

// Meta describes one user-facing check. It aliases the shared engine contract;
// capabilities belong to registry scheduling rather than check metadata.
type Meta = core.Meta

// Check is the shared metadata contract implemented by every check.
type Check = core.Check

type catalogCheck struct {
	meta Meta
}

func (check catalogCheck) Meta() Meta {
	return check.meta
}
