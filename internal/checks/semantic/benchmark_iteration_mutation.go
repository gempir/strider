package semantic

import (
	"go/ast"
	"go/token"

	"github.com/gempir/strider/internal/diagnostic"
)

type benchmarkIterationMutationCheck struct{}

func (benchmarkIterationMutationCheck) Meta() Meta {
	return Meta{
		Code:            "benchmark-iteration-mutation",
		Summary:         "detect assignments to testing.B.N",
		Explanation:     "The testing package dynamically controls B.N to calibrate benchmark duration and calculate per-operation time. Benchmark code that changes N invalidates those measurements.",
		GoodExample:     "for range b.N { operation() }",
		BadExample:      "b.N = 1000",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (benchmarkIterationMutationCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.AssignStmt)(nil),
		},
		func(node ast.Node) bool {
			assignment, ok := node.(*ast.AssignStmt)
			if !ok || assignment.Tok != token.ASSIGN {
				return true
			}
			for _, left := range assignment.Lhs {
				selector, ok := left.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "N" || !isPointerToNamedType(pass.TypesInfo.TypeOf(selector.X), "testing", "B") {
					continue
				}
				pass.Report(selector, "do not assign to testing.B.N; the benchmark runner controls it")
			}
			return true
		},
	)
}

func (benchmarkIterationMutationCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
