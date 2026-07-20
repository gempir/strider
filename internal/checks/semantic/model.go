// Package semantic implements Strider's package-aware check engine.
package semantic

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/checks/core"
	"github.com/gempir/strider/internal/diagnostic"
)

// Meta describes one built-in semantic check.
type Meta = core.Meta

// Rule is a package-aware semantic check.
type Rule interface {
	core.Check
	Run(*Pass)
}

// Pass contains the syntax and type information for one loaded Go package.
type Pass struct {
	PackagePath string
	GoVersion   string
	Files       []*ast.File
	FileSet     *token.FileSet
	Types       *types.Package
	TypesSizes  types.Sizes
	TypesInfo   *types.Info
	SSAProgram  *ssa.Program
	SSAPackage  *ssa.Package
	Functions   []*ssa.Function
	facts       *packageFacts
	maxMethods  int

	deprecatedObjects  map[types.Object]string
	deprecatedPackages map[*types.Package]string

	report func(token.Pos, token.Pos, string, []diagnostic.Fix)
}

// Report emits a diagnostic for the rule currently running.
func (pass *Pass) Report(node ast.Node, message string) {
	pass.ReportPos(node.Pos(), message)
}

// ReportPos emits a diagnostic at pos. It is intended for SSA checks, which
// have source positions but do not always have an AST node to report.
func (pass *Pass) ReportPos(pos token.Pos, message string) {
	pass.report(pos, pos, message, nil)
}

// ReportFix emits a diagnostic with one or more suggested fixes. Edits use
// byte offsets in the diagnostic's source file.
func (pass *Pass) ReportFix(node ast.Node, message string, fixes ...diagnostic.Fix) {
	pass.report(node.Pos(), node.End(), message, fixes)
}
