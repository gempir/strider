package rules

import (
	"go/ast"
	"go/token"
)

func (a *analyzer) checkIncDec(statement *ast.IncDecStmt) {
	if root := rootIdent(statement.X); root != nil {
		a.checkIdentifierName(root)
	}
}

func (a *analyzer) checkBranch(branch *ast.BranchStmt) {
	if branch.Tok == token.BREAK && branch.Label == nil {
		for index := len(a.ancestors) - 1; index >= 0; index-- {
			if clause, ok := a.ancestors[index].(*ast.CaseClause); ok && len(clause.Body) > 0 &&
				clause.Body[len(clause.Body)-1] == branch {
				a.report("useless-break", branch, "break at the end of a switch case is redundant")
				a.report(
					"unnecessary-stmt",
					branch,
					"omit unnecessary break at the end of a case clause",
				)
				break
			}
		}
	}
	if branch.Tok == token.FALLTHROUGH {
		if clause, ok := a.parent().(*ast.CaseClause); ok && len(clause.Body) > 0 &&
			clause.Body[len(clause.Body)-1] != branch {
			a.report(
				"unnecessary-stmt",
				branch,
				"fallthrough should be the final statement in a case",
			)
		}
	}
}
