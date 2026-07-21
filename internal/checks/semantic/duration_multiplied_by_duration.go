package semantic

import (
	"go/ast"
	"go/token"

	"github.com/gempir/strider/internal/diagnostic"
)

type durationMultipliedByDurationCheck struct{}

func (durationMultipliedByDurationCheck) Meta() Meta {
	return Meta{
		Code:            "duration-multiplied-by-duration",
		Summary:         "detect multiplication of two time.Duration values",
		Explanation:     "time.Duration stores a scalar number of nanoseconds. Multiplying two duration values produces squared time units and usually means that a caller supplied an already-scaled duration where a plain count was expected.",
		GoodExample:     "delay := time.Duration(count) * time.Second",
		BadExample:      "delay := duration * time.Second",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (durationMultipliedByDurationCheck) Run(pass *Pass) {
	pass.InspectWithStack(
		[]ast.Node{
			(*ast.AssignStmt)(nil),
			(*ast.BinaryExpr)(nil),
		},
		func(node ast.Node, ancestors []ast.Node) bool {
			var parent ast.Node
			if len(ancestors) != 0 {
				parent = ancestors[len(ancestors)-1]
			}
			var candidate ast.Expr
			switch expression := node.(type) {
			case *ast.BinaryExpr:
				if expression.Op != token.MUL {
					return true
				}
				if binaryParent, ok := parent.(*ast.BinaryExpr); ok && binaryParent.Op == token.MUL {
					return true
				}
				candidate = expression
			case *ast.AssignStmt:
				if expression.Tok != token.MUL_ASSIGN || len(expression.Lhs) != 1 || len(expression.Rhs) != 1 {
					return true
				}
				candidate = &ast.BinaryExpr{
					X: expression.Lhs[0],
					Y: expression.Rhs[0],
				}
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

func semanticDurationFactors(pass *Pass, expression ast.Expr) int {
	switch expression := unparenExpression(expression).(type) {
	case *ast.BinaryExpr:
		left := semanticDurationFactors(pass, expression.X)
		right := semanticDurationFactors(pass, expression.Y)
		switch expression.Op {
		case token.MUL, token.ILLEGAL:
			return left + right
		case token.ADD, token.SUB:
			if left+right != 0 {
				return 1
			}
			return 0
		case token.QUO:
			if left != 0 && right == 0 {
				return 1
			}
			return 0
		default:
			if left+right != 0 && isNamedType(pass.TypesInfo.TypeOf(expression), "time", "Duration") {
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
		if object != nil && isNamedType(object.Type(), "time", "Duration") {
			return 1
		}
		return 0
	case *ast.SelectorExpr:
		object := pass.TypesInfo.ObjectOf(expression.Sel)
		if object != nil && isNamedType(object.Type(), "time", "Duration") {
			return 1
		}
		return 0
	case *ast.CallExpr:
		if isUnitlessDurationConversion(pass, expression) {
			return 0
		}
	}
	if isNamedType(pass.TypesInfo.TypeOf(expression), "time", "Duration") {
		return 1
	}
	return 0
}

func isUnitlessDurationConversion(pass *Pass, expression ast.Expr) bool {
	expression = unparenExpression(expression)
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || !pass.TypesInfo.Types[call.Fun].IsType() || !isNamedType(pass.TypesInfo.TypeOf(call), "time", "Duration") {
		return false
	}
	return semanticDurationFactors(pass, call.Args[0]) == 0
}

func (durationMultipliedByDurationCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
