package rules

import (
	"go/ast"
	"go/token"
	"regexp"
)

var cgoPointerIdentifier = regexp.MustCompile(`^_C(func|var)_.+$`)

func (a *analyzer) checkPointerRoundTrip(node ast.Node) {
	switch expression := node.(type) {
	case *ast.UnaryExpr:
		if expression.Op != token.AND {
			return
		}
		star, ok := expression.X.(*ast.StarExpr)
		if !ok || isCGOPointerIdentifier(star.X) {
			return
		}
		a.report(
			"ineffective-pointer-copy",
			expression,
			"&*value simplifies to value and does not copy the pointed-to data",
		)
	case *ast.StarExpr:
		address, ok := expression.X.(*ast.UnaryExpr)
		if !ok || address.Op != token.AND {
			return
		}
		a.report(
			"ineffective-pointer-copy",
			expression,
			"*&value simplifies to value and does not make a copy",
		)
	}
}

func isCGOPointerIdentifier(expression ast.Expr) bool {
	identifier, ok := expression.(*ast.Ident)
	return ok && cgoPointerIdentifier.MatchString(identifier.Name)
}
