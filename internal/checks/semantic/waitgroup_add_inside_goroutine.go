package semantic

import (
	"go/ast"

	"github.com/gempir/strider/internal/diagnostic"
)

type waitGroupAddInsideGoroutineRule struct{}

func (waitGroupAddInsideGoroutineRule) Meta() Meta {
	return Meta{
		Code:            "waitgroup-add-inside-goroutine",
		Summary:         "detect WaitGroup.Add calls inside newly started goroutines",
		Explanation:     "WaitGroup.Add must happen before starting the goroutine it accounts for. Calling Add inside the goroutine races with Wait, which may observe a zero counter and return too early.",
		GoodExample:     "group.Add(1)\ngo func() { defer group.Done() }()",
		BadExample:      "go func() { group.Add(1); defer group.Done() }()",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (waitGroupAddInsideGoroutineRule) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.GoStmt)(nil),
		},
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
					if ok && isNamedMethod(pass.TypesInfo, call.Fun, "sync", "WaitGroup", "Add") {
						pass.Report(call, "call WaitGroup.Add before starting the goroutine to avoid racing with Wait")
					}
					return true
				},
			)
			return false
		},
	)
}

func (waitGroupAddInsideGoroutineRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
