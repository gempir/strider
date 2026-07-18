package analyze

import (
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type waitGroupAddInsideGoroutineRule struct {}

func (waitGroupAddInsideGoroutineRule) Meta() Meta {
	return Meta{
		Code: "waitgroup-add-inside-goroutine",
		Summary: "detect WaitGroup.Add calls inside newly started goroutines",
		Explanation: "WaitGroup.Add must happen before starting the goroutine it accounts for. Calling Add inside the goroutine races with Wait, which may observe a zero counter and return too early.",
		GoodExample: "group.Add(1)\ngo func() { defer group.Done() }()",
		BadExample: "go func() { group.Add(1); defer group.Done() }()",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (waitGroupAddInsideGoroutineRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				statement,
				ok := node.(*ast.GoStmt)
				if !ok {
					return true
				}
				literal,
				ok := statement.Call.Fun.(*ast.FuncLit)
				if !ok {
					return true
				}
				ast.Inspect(
					literal.Body,
					func(nested ast.Node) bool {
						if nested != literal.Body {
							if _,
							nestedFunction := nested.(*ast.FuncLit); nestedFunction {
								return false
							}
						}
						call,
						ok := nested.(*ast.CallExpr)
						if ok && isWaitGroupMethod(pass.TypesInfo, call.Fun, "Add") {
							pass.Report(
								call,
								"call WaitGroup.Add before starting the goroutine to avoid racing with Wait",
							)
						}
						return true
					},
				)
				return false
			},
		)
	}
}

func isWaitGroupMethod(info *types.Info, expression ast.Expr, name string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != name {
		return false
	}
	function, ok := info.Uses[selector.Sel].(*types.Func)
	if !ok || function.Pkg() == nil || function.Pkg().Path() != "sync" {
		return false
	}
	signature, _ := function.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		return false
	}
	receiver := types.Unalias(signature.Recv().Type())
	if pointer, ok := receiver.(*types.Pointer); ok {
		receiver = types.Unalias(pointer.Elem())
	}
	named, ok := receiver.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "sync" && named.Obj().Name() == "WaitGroup"
}
