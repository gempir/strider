// Package semantic implements Strider's package-aware check engine.
package semantic

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/checks/core"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
)

// Meta describes one built-in semantic check.
type Meta = core.Meta

// Check is a package-aware semantic check.
type Check interface {
	core.Check
	Run(*Pass)
}

// Rule is retained for source compatibility while callers migrate to Check.
// Deprecated: use Check.
type Rule = Check

// configurableRule keeps per-check settings with the check that consumes
// them instead of widening Pass for a single option.
type configurableRule interface {
	RunConfigured(*Pass, config.CheckConfig)
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
	report      func(token.Pos, token.Pos, string, []diagnostic.Fix)
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

// Inspect visits the typed package's syntax once for the supplied callback.
// Typed checks use this shared entry point instead of each owning file loops.
func (pass *Pass) Inspect(visit func(ast.Node) bool) {
	for _, file := range pass.Files {
		ast.Inspect(file, visit)
	}
}
