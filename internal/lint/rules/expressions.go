package rules

import (
	"go/ast"
	"go/token"
	"strings"
)

func (a *analyzer) checkBinary(binary *ast.BinaryExpr) {
	if binary.Op == token.EQL || binary.Op == token.NEQ {
		if booleanLiteral(binary.X) || booleanLiteral(binary.Y) {
			a.report(
				"bool-literal-in-expr",
				binary,
				"omit the boolean literal from the logical expression",
			)
		}
		if looksLikeTime(binary.X) || looksLikeTime(binary.Y) {
			a.report(
				"time-equal",
				binary,
				"compare time.Time values with Equal instead of == or !=",
			)
		}
	}
	if binary.Op == token.LAND || binary.Op == token.LOR {
		if value, ok := staticBool(binary.X); ok {
			if (binary.Op == token.LAND && !value) || (binary.Op == token.LOR && value) {
				a.report(
					"constant-logical-expr",
					binary,
					"logical expression always has the same value",
				)
			}
		}
		if expressionCost(binary.X) > expressionCost(binary.Y) {
			a.report(
				"optimize-operands-order",
				binary,
				"place the cheaper logical operand first to improve short-circuiting",
			)
		}
	}
}

func booleanLiteral(expr ast.Expr) bool {
	id, ok := expr.(*ast.Ident)
	return ok && (id.Name == "true" || id.Name == "false")
}

func staticBool(expr ast.Expr) (bool, bool) {
	id, ok := expr.(*ast.Ident)
	if !ok {
		return false, false
	}
	if id.Name == "true" {
		return true, true
	}
	if id.Name == "false" {
		return false, true
	}
	return false, false
}

func looksLikeTime(expr ast.Expr) bool {
	text := nodeText(expr)
	return strings.Contains(text, "time.Now(") || strings.Contains(text, "time.Date(") ||
		strings.Contains(text, ".Time")
}

func expressionCost(expr ast.Expr) int {
	cost := 0
	ast.Inspect(
		expr,
		func(node ast.Node) bool {
			switch node.(type) {
			case *ast.CallExpr:
				cost += 10
			case *ast.IndexExpr, *ast.SelectorExpr:
				cost += 2
			case *ast.BinaryExpr:
				cost++
			}
			return true
		},
	)
	return cost
}
