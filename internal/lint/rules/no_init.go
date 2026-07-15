package rules

import "go/ast"

func (a *analyzer) checkNoInit(function *ast.FuncDecl) {
	if function.Recv == nil && function.Name.Name == "init" {
		a.report(
			"no-init",
			function.Name,
			"replace init with explicit initialization",
		)
	}
}
