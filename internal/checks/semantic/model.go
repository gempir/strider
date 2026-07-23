// Package semantic implements Strider's package-aware check engine.
package semantic

import (
	"go/ast"
	"go/token"
	"go/types"

	astinspector "golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/checks/catalog"
	"github.com/gempir/strider/internal/diagnostic"
)

// Meta describes one built-in semantic check.
type Meta = catalog.Meta

// Check is a package-aware semantic check.
type Check interface {
	catalog.Check
	Requirements() Requirements
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
	inspector   *astinspector.Inspector
	facts       *packageFacts
	options     catalog.ResolvedOptions
	report      func(token.Pos, token.Pos, string, []diagnostic.Fix)
}

// IntOption returns a schema-bound integer option for the running check.
func (pass *Pass) IntOption(name string) int {
	value, _ := pass.options.Int(name)
	return value
}

// StringsOption returns a schema-bound string-list option for the running check.
func (pass *Pass) StringsOption(name string) []string {
	value, _ := pass.options.Strings(name)
	return value
}

// Report emits a diagnostic for the check currently running.
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

// Inspect visits the typed package's syntax once for the supplied callback.
// Typed checks use this shared entry point instead of each owning file loops.
func (pass *Pass) Inspect(nodeFilter []ast.Node, visit func(ast.Node) bool) {
	inspector := pass.inspector
	if inspector == nil {
		inspector = astinspector.New(pass.Files)
	}
	inspector.Nodes(nodeFilter, func(node ast.Node, push bool) bool {
		if !push {
			return true
		}
		return visit(node)
	})
}

// InspectWithStack visits only the requested node types and supplies their
// complete ancestor stack without requiring a check-owned traversal.
func (pass *Pass) InspectWithStack(nodeFilter []ast.Node, visit func(ast.Node, []ast.Node) bool) {
	inspector := pass.inspector
	if inspector == nil {
		inspector = astinspector.New(pass.Files)
	}
	inspector.WithStack(nodeFilter, func(node ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}
		return visit(node, stack[:len(stack)-1])
	})
}

// File returns the package syntax file containing position.
func (pass *Pass) File(position token.Pos) *ast.File {
	for _, file := range pass.Files {
		if file.Pos() <= position && position <= file.End() {
			return file
		}
	}
	return nil
}
