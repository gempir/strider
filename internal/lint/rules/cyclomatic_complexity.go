package rules

import (
	"fmt"
	"go/ast"
)

func (a *analyzer) checkCyclomaticComplexity(function *ast.FuncDecl) {
	complexity := functionComplexity(function.Body)
	if complexity > 10 {
		a.report(
			"cyclomatic-complexity",
			function.Name,
			fmt.Sprintf("function complexity is %d; maximum is 10", complexity),
		)
	}
}
