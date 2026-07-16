package rules

import (
	"go/ast"
	"go/token"
)

func (a *analyzer) checkDoubleNegation(expression *ast.UnaryExpr) {
	if expression.Op != token.NOT {
		return
	}
	inner, ok := expression.X.(*ast.UnaryExpr)
	if !ok || inner.Op != token.NOT {
		return
	}
	a.report(
		"double-negation",
		expression,
		"double boolean negation has no effect and should be removed",
	)
}
