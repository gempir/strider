package rules

import (
	"github.com/gempir/strider/internal/checks/catalog"
	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/diagnostic"
)

const (
	fileNodeKind   NodeKind = "<file>"
	finishNodeKind NodeKind = "<finish>"
)

// Meta describes one built-in syntax check.
type Meta = catalog.Meta

// NodeKind identifies a CST shape a syntax check consumes. The native engine
// keeps a single traversal and dispatches only the selected interests.
type NodeKind string

// Check is a concrete-syntax check selected by the registry. The
// traversal owns walking the CST; checks declare their metadata here and are
// the only source of enabled syntax work.
type Check interface {
	catalog.Check
	Interests() []NodeKind
	Inspect(*Pass, cst.Node)
}

type definition struct {
	meta     Meta
	behavior syntaxBehavior
}

type syntaxBehavior struct {
	interests []NodeKind
	inspect   func(*Pass, cst.Node)
}

// Finding is a check result before the syntax package converts source positions
// and applies suppression directives.
type Finding struct {
	Node     cst.Node
	Start    int
	End      int
	HasRange bool
	Code     string
	Message  string
	Fixes    []diagnostic.Fix
}

// CSTInput contains everything needed for the concrete-syntax lint pass.
type CSTInput struct {
	Filename string
	Tree     *cst.Tree
	Checks   []Check
	Options  map[string]catalog.ResolvedOptions
	Report   func(Finding)
}

func (check definition) Meta() Meta {
	return catalog.CloneMeta(check.meta)
}

func (check definition) Interests() []NodeKind {
	return append([]NodeKind(nil), check.behavior.interests...)
}

func (check definition) Inspect(pass *Pass, node cst.Node) {
	if check.behavior.inspect != nil {
		check.behavior.inspect(pass, node)
	}
}
