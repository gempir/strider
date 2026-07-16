package rules

import (
	"go/ast"
	"go/token"

	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/diagnostic"
)

// Meta describes one built-in lint rule.
type Meta struct {
	Code            string              `json:"code"`
	Summary         string              `json:"summary"`
	Explanation     string              `json:"explanation"`
	GoodExample     string              `json:"good_example"`
	BadExample      string              `json:"bad_example"`
	DefaultSeverity diagnostic.Severity `json:"default_severity"`
}

// Rule is the common contract used to select, list, explain, and run every
// built-in lint rule.
type Rule interface {
	Meta() Meta
	defaultEnabled() bool
}

type definition struct {
	meta        Meta
	defaultRule bool
}

func (rule definition) Meta() Meta {
	return rule.meta
}

func (rule definition) defaultEnabled() bool {
	return rule.defaultRule
}

// Finding is a rule result before the lint package converts source positions
// and applies suppression directives.
type Finding struct {
	Node              ast.Node
	Scope             ast.Node
	Ancestors         []ast.Node
	ConcreteNode      cst.Node
	ConcreteScope     cst.Node
	ConcreteAncestors []cst.Node
	ConcreteStart     int
	ConcreteEnd       int
	HasConcreteRange  bool
	Code              string
	Message           string
}

// CSTInput contains everything needed for the concrete-syntax lint pass.
type CSTInput struct {
	Filename string
	Tree     *cst.Tree
	Rules    []Rule
	Report   func(Finding)
}

// Input contains everything needed to analyze one parsed Go source file.
type Input struct {
	Filename string
	FileSet  *token.FileSet
	File     *ast.File
	Content  []byte
	Rules    []Rule
	Report   func(Finding)
}
