package semantic

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type durationMultipliedByDurationRule struct {}

func (durationMultipliedByDurationRule) Meta() Meta {
	return Meta{
		Code: "duration-multiplied-by-duration",
		Summary: "detect multiplication of two time.Duration values",
		Explanation: "time.Duration stores a scalar number of nanoseconds. Multiplying two duration values produces squared time units and usually means that a caller supplied an already-scaled duration where a plain count was expected.",
		GoodExample: "delay := time.Duration(count) * time.Second",
		BadExample: "delay := duration * time.Second",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (durationMultipliedByDurationRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		stack := make([]ast.Node, 0)
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				if node == nil {
					stack = stack[:len(stack) - 1]
					return true
				}
				var parent ast.Node
				if len(stack) != 0 {
					parent = stack[len(stack) - 1]
				}
				stack = append(stack, node)
				var candidate ast.Expr
				switch expression := node.(type) {
				case *ast.BinaryExpr:
					if expression.Op != token.MUL {
						return true
					}
					if binaryParent,
					ok := parent.(*ast.BinaryExpr); ok && binaryParent.Op == token.MUL {
						return true
					}
					candidate = expression
				case *ast.AssignStmt:
					if expression.Tok != token.MUL_ASSIGN || len(expression.Lhs) != 1 || len(expression.Rhs) != 1 {
						return true
					}
					candidate = &ast.BinaryExpr{X:
					expression.Lhs[0], Y:
					expression.Rhs[0]}
				default:
					return true
				}
				if semanticDurationFactors(pass, candidate) < 2 {
					return true
				}
				pass.Report(node, "multiplying two time.Duration values produces squared time units; multiply a duration by a unitless count")
				return true
			},
		)
	}
}

func semanticDurationFactors(pass *Pass, expression ast.Expr) int {
	switch expression := unparenExpression(expression).(type) {
	case *ast.BinaryExpr:
		left := semanticDurationFactors(pass, expression.X)
		right := semanticDurationFactors(pass, expression.Y)
		switch expression.Op {
		case token.MUL, token.ILLEGAL:
			return left + right
		case token.ADD, token.SUB:
			if left + right != 0 {
				return 1
			}
			return 0
		case token.QUO:
			if left != 0 && right == 0 {
				return 1
			}
			return 0
		default:
			if left + right != 0 && isTimeDuration(pass.TypesInfo.TypeOf(expression)) {
				return 1
			}
			return 0
		}
	case *ast.BasicLit:
		return 0
	case *ast.UnaryExpr:
		return semanticDurationFactors(pass, expression.X)
	case *ast.Ident:
		object := pass.TypesInfo.ObjectOf(expression)
		if object != nil && isTimeDuration(object.Type()) {
			return 1
		}
		return 0
	case *ast.SelectorExpr:
		object := pass.TypesInfo.ObjectOf(expression.Sel)
		if object != nil && isTimeDuration(object.Type()) {
			return 1
		}
		return 0
	case *ast.CallExpr:
		if isUnitlessDurationConversion(pass, expression) {
			return 0
		}
	}
	if isTimeDuration(pass.TypesInfo.TypeOf(expression)) {
		return 1
	}
	return 0
}

func isUnitlessDurationConversion(pass *Pass, expression ast.Expr) bool {
	expression = unparenExpression(expression)
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || !pass.TypesInfo.Types[call.Fun].IsType() || !isTimeDuration(pass.TypesInfo.TypeOf(call)) {
		return false
	}
	return semanticDurationFactors(pass, call.Args[0]) == 0
}

func isTimeDuration(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	named, ok := types.Unalias(valueType).(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == "time" && named.Obj().Name() == "Duration"
}
