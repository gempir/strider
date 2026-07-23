package semantic

import (
	"go/ast"
	"go/constant"
	"go/token"

	"github.com/gempir/strider/internal/diagnostic"
)

type spinningEmptyLoopCheck struct{}

func (spinningEmptyLoopCheck) Meta() Meta {
	return Meta{
		Code:            "spinning-empty-loop",
		Summary:         "detect empty loops that consume a core while waiting unsafely",
		Explanation:     "An empty unconditional loop spins at full speed. An empty loop that only rereads variables can terminate only through unsynchronized mutation, which is a data race; use synchronization or a blocking operation instead.",
		GoodExample:     "for !ready() {} // condition is dynamically evaluated",
		BadExample:      "for {}",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (spinningEmptyLoopCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.ForStmt)(nil),
		},
		func(node ast.Node) bool {
			loop, ok := node.(*ast.ForStmt)
			if !ok || len(loop.Body.List) != 0 || loop.Init != nil || loop.Post != nil {
				return true
			}
			if loop.Cond == nil {
				pass.Report(loop, "empty unconditional loop spins and consumes a full CPU core")
				return true
			}
			if analysisExpressionHasDynamicEffect(loop.Cond) || constantFalse(pass, loop.Cond) {
				return true
			}
			pass.Report(loop, "empty loop condition cannot change safely; synchronize instead of spinning")
			return true
		},
	)
}

func analysisExpressionHasDynamicEffect(expression ast.Expr) bool {
	found := false
	ast.Inspect(
		expression,
		func(node ast.Node) bool {
			if found {
				return false
			}
			switch node := node.(type) {
			case *ast.CallExpr:
				found = true
				return false
			case *ast.UnaryExpr:
				if node.Op == token.ARROW {
					found = true
					return false
				}
			}
			return true
		},
	)
	return found
}

func constantFalse(pass *Pass, expression ast.Expr) bool {
	value := pass.TypesInfo.Types[expression].Value
	return value != nil && value.Kind() == constant.Bool && !constant.BoolVal(value)
}
