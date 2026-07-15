package rules

import (
	"fmt"
	"go/ast"
	"go/token"
)

func (a *analyzer) checkFor(statement *ast.ForStmt) {
	a.checkControlNesting(statement)
}

func (a *analyzer) checkRange(statement *ast.RangeStmt) {
	a.checkControlNesting(statement)
	if id, ok := statement.Value.(*ast.Ident); ok && id.Name == "_" {
		a.report("range", id, "omit the blank range value")
	}
	for _, expr := range []ast.Expr{statement.Key, statement.Value} {
		id, ok := expr.(*ast.Ident)
		if !ok || id.Name == "_" {
			continue
		}
		a.rangeVariables[id.Name] = true
		ast.Inspect(
			statement.Body,
			func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.UnaryExpr:
					if n.Op == token.AND && expressionContainsIdent(n.X, id.Name) {
						a.report(
							"range-val-address",
							n,
							fmt.Sprintf(
								"taking the address of range value %s can be misleading",
								id.Name,
							),
						)
					}
				case *ast.FuncLit:
					if expressionContainsIdent(n.Body, id.Name) {
						a.report(
							"range-val-in-closure",
							n,
							fmt.Sprintf("closure captures range value %s", id.Name),
						)
						a.report(
							"datarace",
							n,
							fmt.Sprintf(
								"goroutine or closure captures changing range value %s",
								id.Name,
							),
						)
					}
				}
				return true
			},
		)
	}
}

func (a *analyzer) checkControlNesting(node ast.Node) {
	depth := 0
	for _, ancestor := range a.ancestors {
		switch ancestor.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			depth++
		}
	}
	if depth >= 5 {
		a.report("max-control-nesting", node, "control-flow nesting exceeds 5 levels")
	}
}
