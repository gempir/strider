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

func isPointerToNamedType(valueType types.Type, packagePath, name string) bool {
	pointer, _ := types.Unalias(valueType).(*types.Pointer)
	return pointer != nil && isNamedType(pointer.Elem(), packagePath, name)
}

func isNamedReceiverType(valueType types.Type, packagePath, name string) bool {
	named := namedType(valueType)
	return named != nil && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == packagePath && named.Obj().Name() == name
}

func calledMethod(info *types.Info, expression ast.Expr) (*types.Func, *types.Named) {
	selector, _ := ast.Unparen(expression).(*ast.SelectorExpr)
	if selector == nil {
		return nil, nil
	}
	function, _ := info.ObjectOf(selector.Sel).(*types.Func)
	if function == nil {
		return nil, nil
	}
	signature, _ := function.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		return nil, nil
	}
	return function, namedType(signature.Recv().Type())
}

func isNamedMethod(info *types.Info, expression ast.Expr, packagePath, receiverName, methodName string) bool {
	function, receiver := calledMethod(info, expression)
	return function != nil && function.Pkg() != nil && function.Pkg().Path() == packagePath && function.Name() == methodName && receiver != nil && receiver.Obj().Pkg() != nil && receiver.Obj().Pkg().Path() == packagePath && receiver.Obj().Name() == receiverName
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

// inspectFunctionBody visits a function body without descending into nested
// function literals. A nested literal has its own control-flow scope.
func inspectFunctionBody(root ast.Node, visit func(ast.Node) bool) {
	first := true
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		if _, nested := node.(*ast.FuncLit); nested && !first {
			return false
		}
		first = false
		return visit(node)
	})
}

func normalizeGoVersion(value string) string {
	if len(value) >= 2 && value[:2] == "go" {
		return value
	}
	return "go" + value
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
