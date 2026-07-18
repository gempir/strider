// Package semantic implements Strider's package-aware check engine.
package semantic

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

// Meta describes one built-in semantic check.
type Meta struct {
	Code string `json:"code"`
	Summary string `json:"summary"`
	Explanation string `json:"explanation"`
	GoodExample string `json:"good_example"`
	BadExample string `json:"bad_example"`
	DefaultSeverity diagnostic.Severity `json:"default_severity"`
}

// Rule is a package-aware semantic check.
type Rule interface {
	Meta() Meta
	Run(*Pass)
}

// Pass contains the syntax and type information for one loaded Go package.
type Pass struct {
	PackagePath string
	GoVersion string
	Files []*ast.File
	FileSet *token.FileSet
	Types *types.Package
	TypesSizes types.Sizes
	TypesInfo *types.Info
	SSAProgram *ssa.Program
	SSAPackage *ssa.Package
	Functions []*ssa.Function
	facts *packageFacts

	deprecatedObjects map[types.Object]string
	deprecatedPackages map[*types.Package]string

	report func(ast.Node, string)
}

// Report emits a diagnostic for the rule currently running.
func (pass *Pass) Report(node ast.Node, message string) {
	pass.report(node, message)
}
