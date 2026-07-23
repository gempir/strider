// Package checks implements Strider's unified diagnostic pipeline.
//
//strider:ignore-file no-package-var,top-level-declaration-order
package checks

import (
	"github.com/gempir/strider/internal/checks/catalog"
	"github.com/gempir/strider/internal/diagnostic"
)

// Engine identifies the execution engine behind a catalog descriptor.
type Engine string

const (
	EngineFormat   Engine = "format"
	EngineSyntax   Engine = "syntax"
	EngineSemantic Engine = "semantic"
)

var formatMeta = Meta{
	Code:            "format",
	Summary:         "require canonical Strider formatting",
	Explanation:     "Canonical formatting keeps Go source deterministic and removes style-only review noise.",
	GoodExample:     "package main\n\nfunc main() {}",
	BadExample:      "package main\nfunc main( ){ }",
	DefaultSeverity: diagnostic.SeverityWarning,
}

// Meta describes one user-facing check.
type Meta = catalog.Meta

// Descriptor is a unified presentation entry. Execution remains engine
// specific and is never forced through this interface.
type Descriptor interface {
	catalog.Descriptor
	Engine() Engine
}

type Check = Descriptor
