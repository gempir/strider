package semantic

import (
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type deferredReturnFunctionNotCalledRule struct{}

func (deferredReturnFunctionNotCalledRule) Meta() Meta {
	return Meta{
		Code:            "deferred-return-function-not-called",
		Summary:         "detect deferred setup calls whose returned function is not called",
		Explanation:     "A function that returns another function commonly performs setup and returns cleanup work. `defer setup()` defers the setup call itself and discards its returned function; `defer setup()()` runs setup immediately and defers the returned cleanup function.",
		GoodExample:     "defer setup()()",
		BadExample:      "defer setup()",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (deferredReturnFunctionNotCalledRule) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.DeferStmt)(nil),
		},
		func(node ast.Node) bool {
			statement,
				ok := node.(*ast.DeferStmt)
			if !ok || statement.Call == nil {
				return true
			}
			result := pass.TypesInfo.TypeOf(statement.Call)
			if result == nil {
				return true
			}
			if _,
				ok := result.Underlying().(*types.Signature); !ok {
				return true
			}
			pass.Report(statement, "the deferred call returns a function that is never called; use a second call to defer the returned function")
			return true
		},
	)
}

func (deferredReturnFunctionNotCalledRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
