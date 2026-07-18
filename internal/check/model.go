// Package check implements Strider's unified diagnostic pipeline.
package check

import "github.com/gempir/strider/internal/diagnostic"

// Capability describes the most expensive program representation required by
// a check. Capabilities are internal scheduling details, not CLI categories.
type Capability uint8

const (
	CapabilitySource Capability = 1 << iota
	CapabilityCST
	CapabilityAST
	CapabilityTypes
	CapabilityFacts
	CapabilitySSA
)

// Meta describes one user-facing check.
type Meta struct {
	Code string `json:"code"`
	Summary string `json:"summary"`
	Explanation string `json:"explanation"`
	GoodExample string `json:"good_example"`
	BadExample string `json:"bad_example"`
	DefaultSeverity diagnostic.Severity `json:"default_severity"`
	Capabilities Capability `json:"capabilities"`
}

// Rule is a selected check and its metadata.
type Rule struct {
	meta Meta
}

func (rule Rule) Meta() Meta {
	return rule.meta
}

var formatMeta = Meta{
	Code: "format",
	Summary: "require canonical Strider formatting",
	Explanation: "Canonical formatting keeps Go source deterministic and removes style-only review noise.",
	GoodExample: "Run `strider fmt` before committing.",
	BadExample: "Commit source for which `strider check --only format` reports a finding.",
	DefaultSeverity: diagnostic.SeverityWarning,
	Capabilities: CapabilityCST,
}
