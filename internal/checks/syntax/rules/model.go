package rules

import (
	"github.com/gempir/strider/internal/checks/core"
	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/diagnostic"
)

// Meta describes one built-in syntax check.
type Meta = core.Meta

// NodeKind identifies a CST shape a syntax check consumes. The native engine
// keeps a single traversal and dispatches only the selected interests.
type NodeKind string

// SyntaxCheck is a concrete-syntax check selected by the registry. The
// traversal owns walking the CST; checks declare their metadata here and are
// the only source of enabled syntax work.
type SyntaxCheck interface {
	core.Check
	Interests() []NodeKind
	Inspect(*Pass, cst.Node)
}

// Rule is retained as a compatibility alias while callers migrate to the
// product-wide “check” vocabulary.
type Rule = SyntaxCheck

type definition struct {
	meta      Meta
	interests []NodeKind
}

// Pass is the shared lossless traversal context supplied to syntax checks.
// It is intentionally private in behavior while the dispatch migration is in
// progress; checks receive it rather than reaching into package globals.
type Pass = cstAnalyzer

// Finding is a rule result before the syntax package converts source positions
// and applies suppression directives.
type Finding struct {
	ConcreteNode     cst.Node
	ConcreteStart    int
	ConcreteEnd      int
	HasConcreteRange bool
	Code             string
	Message          string
	Fixes            []diagnostic.Fix
}

// CSTInput contains everything needed for the concrete-syntax lint pass.
type CSTInput struct {
	Filename         string
	Tree             *cst.Tree
	Checks           []SyntaxCheck
	BannedCharacters []rune
	Limits           map[string]int
	BlockedImports   []string
	Report           func(Finding)
}

func (rule definition) Meta() Meta {
	return rule.meta
}

func (rule definition) Interests() []NodeKind {
	return append([]NodeKind(nil), rule.interests...)
}

func (definition) Inspect(*Pass, cst.Node) {}
