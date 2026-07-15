package rules

import (
	"go/ast"
	"strings"
)

func (a *analyzer) checkFunctionControl(fn *ast.FuncDecl) {
	if fn.Body == nil {
		return
	}
	receiver := ""
	if fn.Recv != nil && len(fn.Recv.List) > 0 && len(fn.Recv.List[0].Names) > 0 {
		receiver = fn.Recv.List[0].Names[0].Name
	}
	if callsSelf(fn.Body, fn.Name.Name, receiver) &&
		!hasNonRecursiveReturn(fn.Body, fn.Name.Name, receiver) {
		a.report(
			"unconditional-recursion",
			fn.Name,
			"function appears to recurse without a non-recursive return path",
		)
	}
	if fn.Name.Name == "TestMain" && strings.HasSuffix(a.filename, "_test.go") {
		ast.Inspect(
			fn.Body,
			func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if ok && callName(call) == "os.Exit" {
					a.report(
						"redundant-test-main-exit",
						call,
						"TestMain can return instead of calling os.Exit",
					)
				}
				return true
			},
		)
	}
}

func callsSelf(node ast.Node, name, receiver string) bool {
	found := false
	ast.Inspect(
		node,
		func(child ast.Node) bool {
			call, ok := child.(*ast.CallExpr)
			if ok && selfCall(call, name, receiver) {
				found = true
			}
			return !found
		},
	)
	return found
}

func selfCall(call *ast.CallExpr, name, receiver string) bool {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return receiver == "" && fn.Name == name
	case *ast.SelectorExpr:
		id, ok := fn.X.(*ast.Ident)
		return receiver != "" && ok && id.Name == receiver && fn.Sel.Name == name
	}
	return false
}

func hasNonRecursiveReturn(body *ast.BlockStmt, name, receiver string) bool {
	found := false
	ast.Inspect(
		body,
		func(node ast.Node) bool {
			ret, ok := node.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			for _, result := range ret.Results {
				if callsSelf(result, name, receiver) {
					return true
				}
			}
			found = true
			return false
		},
	)
	return found
}
