package semantic

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"go/version"
	"sort"
	"strings"

	"github.com/gempir/strider/internal/diagnostic"
)

type timeValueEqualityRule struct{}

type waitGroupGoForbiddenCallRule struct{}

type rangeValueCaptureRule struct{}

func (timeValueEqualityRule) Meta() Meta {
	return Meta{
		Code:            "time-value-equality",
		Summary:         "compare time.Time values with Time.Equal",
		Explanation:     "The == and != operators compare every field of time.Time, including monotonic-clock and location representation details. Time.Equal compares the represented instants and is the intended equality operation.",
		GoodExample:     "if left.Equal(right) { use(left) }",
		BadExample:      "if left == right { use(left) }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (timeValueEqualityRule) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.BinaryExpr)(nil),
		},
		func(node ast.Node) bool {
			binary,
				ok := node.(*ast.BinaryExpr)
			if !ok || (binary.Op != token.EQL && binary.Op != token.NEQ) {
				return true
			}
			if !isNamedType(pass.TypesInfo.TypeOf(binary.X), "time", "Time") || !isNamedType(pass.TypesInfo.TypeOf(binary.Y), "time", "Time") {
				return true
			}
			pass.Report(binary, "compare time.Time values with Time.Equal instead of == or !=")
			return true
		},
	)
}

func (waitGroupGoForbiddenCallRule) Meta() Meta {
	return Meta{
		Code:            "waitgroup-go-forbidden-call",
		Summary:         "reject panic, recover, and WaitGroup.Done inside WaitGroup.Go",
		Explanation:     "sync.WaitGroup.Go calls Done automatically when its task returns, and its contract requires the task not to panic. Calling Done manually corrupts the counter; panic and recover indicate a task that does not satisfy the API contract.",
		GoodExample:     "group.Go(func() { work() })",
		BadExample:      "group.Go(func() { defer group.Done(); panic(\"failed\") })",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (waitGroupGoForbiddenCallRule) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
		},
		func(node ast.Node) bool {
			call,
				ok := node.(*ast.CallExpr)
			if !ok || !isNamedMethod(pass.TypesInfo, call.Fun, "sync", "WaitGroup", "Go") || len(call.Args) == 0 {
				return true
			}
			closure,
				ok := ast.Unparen(call.Args[len(call.Args)-1]).(*ast.FuncLit)
			if !ok || closure.Body == nil {
				return true
			}
			inspectFunctionBody(
				closure.Body,
				func(nested ast.Node) bool {
					forbidden,
						ok := nested.(*ast.CallExpr)
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

func (rangeValueCaptureRule) Meta() Meta {
	return Meta{
		Code:            "range-value-capture",
		Summary:         "detect closures that capture reused range variables",
		Explanation:     "Before Go 1.22, variables declared by a range clause were reused across iterations; variables assigned with = are still reused on every Go version. A closure that outlives an iteration can therefore observe a later value. Pass the value as an argument or make an iteration-local copy.",
		GoodExample:     "for _, value := range values { go func(current int) { use(current) }(value) }",
		BadExample:      "var value int\nfor _, value = range values { callbacks = append(callbacks, func() { use(value) }) }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (rangeValueCaptureRule) Run(pass *Pass) {
	modernRangeVariables := pass.GoVersion != "" && version.Compare(normalizeGoVersion(pass.GoVersion), "go1.22") >= 0
	parents := pass.analysisParents()
	pass.Inspect(
		[]ast.Node{
			(*ast.RangeStmt)(nil),
		},
		func(node ast.Node) bool {
			loop,
				ok := node.(*ast.RangeStmt)
			if !ok || loop.Body == nil || (loop.Tok == token.DEFINE && modernRangeVariables) {
				return true
			}
			variables := rangeVariableObjects(pass, loop)
			if len(variables) == 0 {
				return true
			}
			ast.Inspect(
				loop.Body,
				func(candidate ast.Node) bool {
					closure,
						ok := candidate.(*ast.FuncLit)
					if !ok {
						return true
					}
					if immediatelyInvokedClosure(closure, parents) {
						return true
					}
					captured := directlyCapturedRangeVariables(pass, closure, variables)
					if len(captured) != 0 {
						pass.Report(
							closure,
							fmt.Sprintf("closure captures reused range variable%s %s", pluralSuffix(len(captured)), strings.Join(captured, ", ")),
						)
					}
					return true
				},
			)
			return true
		},
	)
}

func rangeVariableObjects(pass *Pass, loop *ast.RangeStmt) map[types.Object]string {
	result := make(map[types.Object]string, 2)
	for _, expression := range []ast.Expr{
		loop.Key,
		loop.Value,
	} {
		identifier, ok := ast.Unparen(expression).(*ast.Ident)
		if !ok || identifier.Name == "_" {
			continue
		}
		object := pass.TypesInfo.ObjectOf(identifier)
		if object != nil {
			result[object] = identifier.Name
		}
	}
	return result
}

func directlyCapturedRangeVariables(pass *Pass, closure *ast.FuncLit, variables map[types.Object]string) []string {
	captured := make(map[string]bool, len(variables))
	inspectFunctionBody(
		closure.Body,
		func(node ast.Node) bool {
			identifier,
				ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			if name := variables[pass.TypesInfo.ObjectOf(identifier)]; name != "" {
				captured[name] = true
			}
			return true
		},
	)
	result := make([]string, 0, len(captured))
	for name := range captured {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func immediatelyInvokedClosure(closure *ast.FuncLit, parents map[ast.Node]ast.Node) bool {
	var expression ast.Expr = closure
	parent := parents[expression]
	for {
		parenthesized, ok := parent.(*ast.ParenExpr)
		if !ok || parenthesized.X != expression {
			break
		}
		expression = parenthesized
		parent = parents[parenthesized]
	}
	call, ok := parent.(*ast.CallExpr)
	if !ok || call.Fun != expression {
		return false
	}
	switch parents[call].(type) {
	case *ast.GoStmt, *ast.DeferStmt:
		return false
	default:
		return true
	}
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func (rangeValueCaptureRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
		Facts: FactParents,
	}
}

func (timeValueEqualityRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}

func (waitGroupGoForbiddenCallRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
