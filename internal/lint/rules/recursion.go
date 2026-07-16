package rules

import (
	"go/ast"
	"strings"
)

func (a *analyzer) checkFunctionControl(fn *ast.FuncDecl) {
	if fn.Body == nil {
		return
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
