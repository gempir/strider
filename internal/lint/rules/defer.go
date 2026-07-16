package rules

import "go/ast"

func (a *analyzer) checkDefer(statement *ast.DeferStmt) {
	a.checkNoDeferInLoop(statement)
	call := statement.Call
	if call == nil {
		return
	}
	if callName(call) == "recover" {
		a.report("defer", statement, "defer recover() evaluates recover immediately")
	}
	if literal, ok := call.Fun.(*ast.FuncLit); ok && literal.Type.Results != nil &&
		fieldCount(literal.Type.Results) > 0 {
		a.report("defer", statement, "return values from a deferred function are ignored")
	}
}
