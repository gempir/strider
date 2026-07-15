package rules

import "go/ast"

func (a *analyzer) checkNoDeferInLoop(statement *ast.DeferStmt) {
	for index := len(a.ancestors) - 1; index >= 0; index-- {
		switch a.ancestors[index].(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return
		case *ast.ForStmt, *ast.RangeStmt:
			a.report(
				"no-defer-in-loop",
				statement,
				"defer inside a loop runs at function exit, not iteration exit",
			)
			return
		}
	}
}
