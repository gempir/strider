//strider:ignore-file cognitive-complexity,cyclomatic-complexity
package semantic

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type deferCloseBeforeErrorCheckCheck struct{}

func (deferCloseBeforeErrorCheckCheck) Meta() Meta {
	return Meta{
		Code:            "defer-close-before-error-check",
		Summary:         "detect deferred Close calls scheduled before checking acquisition errors",
		Explanation:     "A resource-returning call may yield an unusable or nil value when it also returns an error. Check the error before deferring Close on the resource.",
		GoodExample:     "file, err := os.Open(path); if err != nil { return err }; defer file.Close()",
		BadExample:      "file, err := os.Open(path); defer file.Close(); if err != nil { return err }",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (deferCloseBeforeErrorCheckCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.BlockStmt)(nil),
		},
		func(node ast.Node) bool {
			block, ok := node.(*ast.BlockStmt)
			if !ok {
				return true
			}
			for index := 0; index+1 < len(block.List); index++ {
				assignment, ok := block.List[index].(*ast.AssignStmt)
				if !ok || len(assignment.Rhs) != 1 || len(assignment.Lhs) < 2 {
					continue
				}
				if ignored, ok := assignment.Lhs[len(assignment.Lhs)-1].(*ast.Ident); ok && ignored.Name == "_" {
					continue
				}
				call, ok := ast.Unparen(assignment.Rhs[0]).(*ast.CallExpr)
				if !ok || !callReturnsValueAndError(pass, call) {
					continue
				}
				resource, ok := assignment.Lhs[0].(*ast.Ident)
				if !ok {
					continue
				}
				deferred, ok := block.List[index+1].(*ast.DeferStmt)
				if !ok || deferred.Call == nil {
					continue
				}
				selector, ok := deferred.Call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Close" {
					continue
				}
				receiver := selectorRootIdentifier(selector.X)
				if receiver == nil || pass.TypesInfo.ObjectOf(receiver) != pass.TypesInfo.ObjectOf(resource) {
					continue
				}
				pass.Report(deferred, fmt.Sprintf("check the error from %s before deferring %s.Close", renderAnalysisExpression(pass, call.Fun), resource.Name))
			}
			return true
		},
	)
}

func callReturnsValueAndError(pass *Pass, call *ast.CallExpr) bool {
	signature, ok := pass.TypesInfo.TypeOf(call.Fun).(*types.Signature)
	if !ok || signature.Results().Len() < 2 {
		return false
	}
	last := signature.Results().At(signature.Results().Len() - 1).Type()
	return types.Identical(last, types.Universe.Lookup("error").Type())
}

func selectorRootIdentifier(expression ast.Expr) *ast.Ident {
	switch expression := ast.Unparen(expression).(type) {
	case *ast.Ident:
		return expression
	case *ast.SelectorExpr:
		return selectorRootIdentifier(expression.X)
	default:
		return nil
	}
}
