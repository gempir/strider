package rules

import "go/ast"

func (a *analyzer) checkNoNakedReturn(statement *ast.ReturnStmt) {
	if len(statement.Results) != 0 || !enclosingFunctionHasNamedResults(a.ancestors) {
		return
	}
	a.report("no-naked-return", statement, "return values must be explicit")
}

func enclosingFunctionHasNamedResults(ancestors []ast.Node) bool {
	for index := len(ancestors) - 1; index >= 0; index-- {
		var results *ast.FieldList
		switch function := ancestors[index].(type) {
		case *ast.FuncDecl:
			results = function.Type.Results
		case *ast.FuncLit:
			results = function.Type.Results
		default:
			continue
		}
		if results == nil {
			return false
		}
		for _, field := range results.List {
			if len(field.Names) != 0 {
				return true
			}
		}
		return false
	}
	return false
}
