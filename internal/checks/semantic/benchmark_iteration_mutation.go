package semantic

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type benchmarkIterationMutationRule struct{}

func (benchmarkIterationMutationRule) Meta() Meta {
	return Meta{
		Code:            "benchmark-iteration-mutation",
		Summary:         "detect assignments to testing.B.N",
		Explanation:     "The testing package dynamically controls B.N to calibrate benchmark duration and calculate per-operation time. Benchmark code that changes N invalidates those measurements.",
		GoodExample:     "for range b.N { operation() }",
		BadExample:      "b.N = 1000",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (benchmarkIterationMutationRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				assignment,
					ok := node.(*ast.AssignStmt)
				if !ok || assignment.Tok != token.ASSIGN {
					return true
				}
				for _, left := range assignment.Lhs {
					selector,
						ok := left.(*ast.SelectorExpr)
					if !ok || selector.Sel.Name != "N" || !isTestingB(pass, selector.X) {
						continue
					}
					pass.Report(
						selector,
						"do not assign to testing.B.N; the benchmark runner controls it",
					)
				}
				return true
			},
		)
	}
}

func isTestingB(pass *Pass, expression ast.Expr) bool {
	valueType := types.Unalias(pass.TypesInfo.TypeOf(expression))
	pointer, ok := valueType.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(pointer.Elem()).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "testing" && named.Obj().Name() == "B"
}
