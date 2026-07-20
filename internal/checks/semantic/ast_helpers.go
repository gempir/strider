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

func isNamedType(valueType types.Type, packagePath, name string) bool {
	named, _ := types.Unalias(valueType).(*types.Named)
	return named != nil && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == packagePath && named.Obj().Name() == name
}

// namedType removes aliases and pointer receiver layers before returning the
// declared type shared by receiver and argument checks.
func namedType(valueType types.Type) *types.Named {
	valueType = types.Unalias(valueType)
	for {
		pointer, ok := valueType.(*types.Pointer)
		if !ok {
			break
		}
		valueType = types.Unalias(pointer.Elem())
	}
	named, _ := valueType.(*types.Named)
	return named
}
