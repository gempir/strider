package rules

import "go/ast"

func (a *analyzer) parent() ast.Node {
	if len(a.ancestors) == 0 {
		return nil
	}
	return a.ancestors[len(a.ancestors)-1]
}

func callName(call *ast.CallExpr) string {
	if call == nil {
		return ""
	}
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return exprText(fn.X) + "." + fn.Sel.Name
	}
	return ""
}

func expressionContainsIdent(expr ast.Node, name string) bool {
	found := false
	ast.Inspect(
		expr,
		func(node ast.Node) bool {
			if id, ok := node.(*ast.Ident); ok && id.Name == name {
				found = true
			}
			return !found
		},
	)
	return found
}
