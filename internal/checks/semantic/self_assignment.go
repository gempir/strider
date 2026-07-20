package semantic

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"

	"github.com/gempir/strider/internal/diagnostic"
)

type selfAssignmentCheck struct{}

func (selfAssignmentCheck) Meta() Meta {
	return Meta{
		Code:            "self-assignment",
		Summary:         "detect assignments that store an expression back into itself",
		Explanation:     "Assigning a side-effect-free expression to the identical destination does nothing and usually indicates a mistaken variable on one side. Expressions with effectful calls or receives are excluded.",
		GoodExample:     "current = next",
		BadExample:      "current = current",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (selfAssignmentCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.AssignStmt)(nil),
		},
		func(node ast.Node) bool {
			assignment, ok := node.(*ast.AssignStmt)
			if !ok || assignment.Tok != token.ASSIGN || len(assignment.Lhs) != len(assignment.Rhs) {
				return true
			}
			for index, left := range assignment.Lhs {
				right := assignment.Rhs[index]
				if reflect.TypeOf(left) != reflect.TypeOf(right) || renderAnalysisExpression(pass, left) != renderAnalysisExpression(pass, right) || !sideEffectFreeExpression(
					pass,
					left,
				) || !sideEffectFreeExpression(pass, right) {
					continue
				}
				text := renderAnalysisExpression(pass, left)
				pass.Report(assignment, fmt.Sprintf("self-assignment of %s has no effect", text))
			}
			return true
		},
	)
}

func sideEffectFreeExpression(pass *Pass, expression ast.Expr) bool {
	safe := true
	ast.Inspect(
		expression,
		func(node ast.Node) bool {
			if !safe {
				return false
			}
			switch node := node.(type) {
			case *ast.UnaryExpr:
				if node.Op == token.ARROW {
					safe = false
					return false
				}
			case *ast.CallExpr:
				if pass.TypesInfo.Types[node.Fun].IsType() || pureBuiltinCall(node) {
					return true
				}
				function := calledFunction(pass.TypesInfo, node.Fun)
				if function == nil {
					safe = false
					return false
				}
				if knownPureFunction(function) {
					return true
				}
				safe = false
				return false
			}
			return true
		},
	)
	return safe
}

func pureBuiltinCall(call *ast.CallExpr) bool {
	identifier, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	switch identifier.Name {
	case "len", "cap", "real", "imag", "complex", "min", "max":
		return true
	default:
		return false
	}
}

func (selfAssignmentCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageSSA,
	}
}
