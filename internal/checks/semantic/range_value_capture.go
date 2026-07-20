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

type rangeValueCaptureCheck struct{}

func (rangeValueCaptureCheck) Meta() Meta {
	return Meta{
		Code:            "range-value-capture",
		Summary:         "detect closures that capture reused range variables",
		Explanation:     "Before Go 1.22, variables declared by a range clause were reused across iterations; variables assigned with = are still reused on every Go version. A closure that outlives an iteration can therefore observe a later value. Pass the value as an argument or make an iteration-local copy.",
		GoodExample:     "for _, value := range values { go func(current int) { use(current) }(value) }",
		BadExample:      "var value int\nfor _, value = range values { callbacks = append(callbacks, func() { use(value) }) }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (rangeValueCaptureCheck) Run(pass *Pass) {
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

func (rangeValueCaptureCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
		Facts: FactParents,
	}
}
