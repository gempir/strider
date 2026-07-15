package rules

import (
	"fmt"
	"go/ast"
)

func (a *analyzer) checkFunctionMutation(fn *ast.FuncDecl) {
	if fn.Body == nil {
		return
	}
	params := map[string]bool{}
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			for _, name := range field.Names {
				params[name.Name] = true
			}
		}
	}
	receivers := map[string]bool{}
	valueReceiver := false
	if fn.Recv != nil {
		for _, field := range fn.Recv.List {
			_, pointer := field.Type.(*ast.StarExpr)
			valueReceiver = valueReceiver || !pointer
			for _, name := range field.Names {
				receivers[name.Name] = true
			}
		}
	}
	ast.Inspect(
		fn.Body,
		func(node ast.Node) bool {
			var targets []ast.Expr
			switch n := node.(type) {
			case *ast.AssignStmt:
				targets = n.Lhs
			case *ast.IncDecStmt:
				targets = []ast.Expr{n.X}
			}
			for _, target := range targets {
				root := rootIdent(target)
				if root != nil && params[root.Name] {
					a.report(
						"modifies-parameter",
						target,
						fmt.Sprintf("assignment modifies parameter %s", root.Name),
					)
				}
				if valueReceiver && root != nil && receivers[root.Name] {
					a.report(
						"modifies-value-receiver",
						target,
						fmt.Sprintf("assignment modifies value receiver %s", root.Name),
					)
				}
			}
			return true
		},
	)
}

func rootIdent(expr ast.Expr) *ast.Ident {
	switch n := expr.(type) {
	case *ast.Ident:
		return n
	case *ast.SelectorExpr:
		return rootIdent(n.X)
	case *ast.IndexExpr:
		return rootIdent(n.X)
	case *ast.StarExpr:
		return rootIdent(n.X)
	}
	return nil
}
