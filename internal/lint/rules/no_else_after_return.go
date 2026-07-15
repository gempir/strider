package rules

import "go/ast"

func (a *analyzer) checkNoElseAfterReturn(statement *ast.IfStmt) {
	if statement.Else == nil || len(statement.Body.List) == 0 {
		return
	}
	if _, ok := statement.Body.List[len(statement.Body.List)-1].(*ast.ReturnStmt); ok {
		a.report(
			"no-else-after-return",
			statement.Else,
			"remove else and unindent its body after the return",
		)
	}
}
