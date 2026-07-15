package rules

import (
	"fmt"
	"go/ast"
)

func (a *analyzer) checkMaxParameters(function *ast.FuncDecl) {
	count := fieldCount(function.Type.Params)
	if count > 5 {
		a.report(
			"max-parameters",
			function.Name,
			fmt.Sprintf("function has %d parameters; maximum is 5", count),
		)
	}
}
