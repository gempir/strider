//strider:ignore-file cognitive-complexity,cyclomatic-complexity
package semantic

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type discardedErrorResultCheck struct{}

func (discardedErrorResultCheck) Meta() Meta {
	return Meta{
		Code:            "discarded-error-result",
		Summary:         "detect discarded error results from typed calls",
		Explanation:     "Ignoring an error result hides a failed operation from callers and can let execution continue with incomplete state. Handle or return actionable errors; conventional fmt output calls and writers whose contracts cannot return errors are excluded.",
		GoodExample:     "value, err := load(); if err != nil { return err }",
		BadExample:      "value, _ := load()",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (discardedErrorResultCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.AssignStmt)(nil),
			(*ast.ExprStmt)(nil),
			(*ast.ValueSpec)(nil),
		},
		func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.ExprStmt:
				call, ok := ast.Unparen(node.X).(*ast.CallExpr)
				if !ok || len(discardedErrorResultIndexes(pass, call)) == 0 {
					return true
				}
				reportDiscardedErrorResult(pass, call)
			case *ast.AssignStmt:
				reportBlankErrorResults(pass, node.Lhs, node.Rhs)
			case *ast.ValueSpec:
				left := make([]ast.Expr, len(node.Names))
				for index, name := range node.Names {
					left[index] = name
				}
				reportBlankErrorResults(pass, left, node.Values)
			}
			return true
		},
	)
}

func reportBlankErrorResults(pass *Pass, left, right []ast.Expr) {
	if len(right) == 1 {
		call, ok := ast.Unparen(right[0]).(*ast.CallExpr)
		if !ok {
			return
		}
		for _, index := range discardedErrorResultIndexes(pass, call) {
			if index < len(left) && blankIdentifier(left[index]) {
				reportDiscardedErrorResult(pass, call)
				return
			}
		}
		return
	}
	if len(left) != len(right) {
		return
	}
	for index, expression := range right {
		if !blankIdentifier(left[index]) {
			continue
		}
		call, ok := ast.Unparen(expression).(*ast.CallExpr)
		if !ok {
			continue
		}
		results := discardedErrorResultIndexes(pass, call)
		if len(results) == 1 && results[0] == 0 && callResultCount(pass, call) == 1 {
			reportDiscardedErrorResult(pass, call)
		}
	}
}

func discardedErrorResultIndexes(pass *Pass, call *ast.CallExpr) []int {
	if discardedErrorResultIsInfallible(pass, call) {
		return nil
	}
	signature := callSignature(pass, call)
	if signature == nil {
		return nil
	}
	indexes := make([]int, 0, 1)
	for index := range signature.Results().Len() {
		if discardedResultIsError(signature.Results().At(index).Type()) {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func discardedErrorResultIsInfallible(pass *Pass, call *ast.CallExpr) bool {
	if isPackageFunction(pass.TypesInfo, call.Fun, "fmt", "Fprint") || isPackageFunction(pass.TypesInfo, call.Fun, "fmt", "Fprintf") || isPackageFunction(
		pass.TypesInfo,
		call.Fun,
		"fmt",
		"Fprintln",
	) {
		return true
	}
	if isPackageFunction(pass.TypesInfo, call.Fun, "io", "WriteString") {
		return len(call.Args) != 0 && infallibleWriterType(pass.TypesInfo.TypeOf(call.Args[0]))
	}
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	return ok && infallibleWriterType(pass.TypesInfo.TypeOf(selector.X))
}

func infallibleWriterType(valueType types.Type) bool {
	named := namedType(valueType)
	if named == nil || named.Obj().Pkg() == nil {
		return false
	}
	packagePath := named.Obj().Pkg().Path()
	name := named.Obj().Name()
	return packagePath == "bytes" && name == "Buffer" || packagePath == "strings" && name == "Builder" || packagePath == "hash" && (name == "Hash" || name == "Hash32" || name == "Hash64")
}

func discardedResultIsError(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	return types.AssignableTo(valueType, types.Universe.Lookup("error").Type())
}

func callResultCount(pass *Pass, call *ast.CallExpr) int {
	signature := callSignature(pass, call)
	if signature == nil {
		return 0
	}
	return signature.Results().Len()
}

func callSignature(pass *Pass, call *ast.CallExpr) *types.Signature {
	valueType := pass.TypesInfo.TypeOf(call.Fun)
	if valueType == nil {
		return nil
	}
	signature, _ := types.Unalias(valueType).Underlying().(*types.Signature)
	return signature
}

func blankIdentifier(expression ast.Expr) bool {
	identifier, ok := ast.Unparen(expression).(*ast.Ident)
	return ok && identifier.Name == "_"
}

func reportDiscardedErrorResult(pass *Pass, call *ast.CallExpr) {
	name := renderAnalysisExpression(pass, call.Fun)
	if name == "" {
		name = "call"
	}
	pass.Report(call, fmt.Sprintf("error result returned by %s is discarded", name))
}
