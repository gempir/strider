package rules

import "go/ast"

func (a *analyzer) checkDefer(statement *ast.DeferStmt) {
	a.checkNoDeferInLoop(statement)
	call := statement.Call
	if call == nil {
		return
	}
	if _, ok := call.Fun.(*ast.CallExpr); ok {
		a.report("defer", statement, "only the final call in a deferred call chain is deferred")
	}
	if callName(call) == "recover" {
		a.report("defer", statement, "defer recover() evaluates recover immediately")
	}
	for index := len(a.ancestors) - 1; index >= 0; index-- {
		switch a.ancestors[index].(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			index = -1
		case *ast.ForStmt, *ast.RangeStmt:
			a.report(
				"defer",
				statement,
				"defer inside a loop runs at function exit, not iteration exit",
			)
			index = -1
		}
	}
	if literal, ok := call.Fun.(*ast.FuncLit); ok && literal.Type.Results != nil &&
		fieldCount(literal.Type.Results) > 0 {
		a.report("defer", statement, "return values from a deferred function are ignored")
	}
}
