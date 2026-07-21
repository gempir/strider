package semantic

import (
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type waitGroupGoForbiddenCallCheck struct{}

func (waitGroupGoForbiddenCallCheck) Meta() Meta {
	return Meta{
		Code:            "waitgroup-go-forbidden-call",
		Summary:         "reject panic, recover, and WaitGroup.Done inside WaitGroup.Go",
		Explanation:     "sync.WaitGroup.Go calls Done automatically when its task returns, and its contract requires the task not to panic. Calling Done manually corrupts the counter; panic and recover indicate a task that does not satisfy the API contract.",
		GoodExample:     "group.Go(func() { work() })",
		BadExample:      "group.Go(func() { defer group.Done(); panic(\"failed\") })",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (waitGroupGoForbiddenCallCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
		},
		func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !isNamedMethod(pass.TypesInfo, call.Fun, "sync", "WaitGroup", "Go") || len(call.Args) == 0 {
				return true
			}
			closure, ok := ast.Unparen(call.Args[len(call.Args)-1]).(*ast.FuncLit)
			if !ok || closure.Body == nil {
				return true
			}
			inspectFunctionBody(
				closure.Body,
				func(nested ast.Node) bool {
					forbidden, ok := nested.(*ast.CallExpr)
					if !ok {
						return true
					}
					switch {
					case isBuiltinCall(pass.TypesInfo, forbidden, "panic"):
						pass.Report(forbidden, "panic must not be called inside sync.WaitGroup.Go")
					case isBuiltinCall(pass.TypesInfo, forbidden, "recover"):
						pass.Report(forbidden, "recover must not be called inside sync.WaitGroup.Go")
					case isNamedMethod(pass.TypesInfo, forbidden.Fun, "sync", "WaitGroup", "Done"):
						pass.Report(forbidden, "sync.WaitGroup.Go calls Done automatically; remove the manual Done call")
					}
					return true
				},
			)
			return true
		},
	)
}

func isBuiltinCall(info *types.Info, call *ast.CallExpr, name string) bool {
	identifier, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := info.Uses[identifier].(*types.Builtin)
	return ok && builtin.Name() == name
}

func (waitGroupGoForbiddenCallCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
