package semantic

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type constantNegativeZeroRule struct{}

func (constantNegativeZeroRule) Meta() Meta {
	return Meta{
		Code:            "constant-negative-zero",
		Summary:         "detect constant expressions that cannot represent negative zero",
		Explanation:     "Go's ideal constants do not preserve a zero sign. Literal forms such as -0.0 and float64(-0) therefore produce positive zero at runtime; use math.Copysign when a true IEEE negative zero is required.",
		GoodExample:     "negativeZero := math.Copysign(0, -1)",
		BadExample:      "negativeZero := -0.0",
		DefaultSeverity: diagnostic.SeverityNote,
	}
}

func (constantNegativeZeroRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				expression,
					ok := node.(ast.Expr)
				if ok && constantNegativeZero(pass, expression) {
					pass.Report(expression, "Go constants cannot represent negative zero; use math.Copysign(0, -1)")
					return false
				}
				return true
			},
		)
	}
}

func constantNegativeZero(pass *Pass, expression ast.Expr) bool {
	switch expression := ast.Unparen(expression).(type) {
	case *ast.UnaryExpr:
		if expression.Op != token.SUB {
			return false
		}
		if literal, ok := ast.Unparen(expression.X).(*ast.BasicLit); ok {
			return literal.Kind == token.FLOAT && zeroLiteral(pass, literal)
		}
		call, ok := ast.Unparen(expression.X).(*ast.CallExpr)
		return ok && floatingConversion(pass, call) && len(call.Args) == 1 && zeroNumericLiteral(pass, call.Args[0])
	case *ast.CallExpr:
		if !floatingConversion(pass, expression) || len(expression.Args) != 1 {
			return false
		}
		negation, ok := ast.Unparen(expression.Args[0]).(*ast.UnaryExpr)
		return ok && negation.Op == token.SUB && zeroIntegerLiteral(pass, negation.X)
	default:
		return false
	}
}

func floatingConversion(pass *Pass, call *ast.CallExpr) bool {
	identifier, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	typeName, ok := pass.TypesInfo.ObjectOf(identifier).(*types.TypeName)
	return ok && typeName.Pkg() == nil && (typeName.Name() == "float32" || typeName.Name() == "float64")
}

func zeroNumericLiteral(pass *Pass, expression ast.Expr) bool {
	literal, ok := ast.Unparen(expression).(*ast.BasicLit)
	return ok && (literal.Kind == token.INT || literal.Kind == token.FLOAT) && zeroLiteral(pass, literal)
}

func zeroIntegerLiteral(pass *Pass, expression ast.Expr) bool {
	literal, ok := ast.Unparen(expression).(*ast.BasicLit)
	return ok && literal.Kind == token.INT && zeroLiteral(pass, literal)
}

func zeroLiteral(pass *Pass, literal *ast.BasicLit) bool {
	value := pass.TypesInfo.Types[literal].Value
	return value != nil && (value.Kind() == constant.Int || value.Kind() == constant.Float) && constant.Sign(value) == 0
}
