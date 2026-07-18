package semantic

import (
	"go/ast"
	"go/types"
)

func calledFunction(info *types.Info, expression ast.Expr) *types.Func {
	switch expression := expression.(type) {
	case *ast.Ident:
		function, _ := info.Uses[expression].(*types.Func)
		return function
	case *ast.SelectorExpr:
		function, _ := info.Uses[expression.Sel].(*types.Func)
		return function
	case *ast.IndexExpr:
		return calledFunction(info, expression.X)
	case *ast.IndexListExpr:
		return calledFunction(info, expression.X)
	case *ast.ParenExpr:
		return calledFunction(info, expression.X)
	default:
		return nil
	}
}

func isPackageFunction(info *types.Info, expression ast.Expr, packagePath, name string) bool {
	function := calledFunction(info, expression)
	return function != nil && function.Pkg() != nil && function.Pkg().Path() == packagePath && function.Name() == name
}
