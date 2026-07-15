package rules

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

func (a *analyzer) checkAssignment(statement *ast.AssignStmt) {
	if len(statement.Lhs) == 1 && len(statement.Rhs) == 1 {
		left, right := nodeText(statement.Lhs[0]), statement.Rhs[0]
		if binary, ok := right.(*ast.BinaryExpr); ok && nodeText(binary.X) == left {
			if (statement.Tok == token.ASSIGN || statement.Tok == token.DEFINE) &&
				(binary.Op == token.ADD || binary.Op == token.SUB) &&
				isOne(binary.Y) {
				a.report(
					"increment-decrement",
					statement,
					"use ++ or -- instead of assigning an addition or subtraction of one",
				)
			}
		}
		if call, ok := right.(*ast.CallExpr); ok {
			name := callName(call)
			if strings.HasPrefix(name, "atomic.") && len(call.Args) > 0 &&
				strings.TrimPrefix(nodeText(call.Args[0]), "&") == left {
				a.report(
					"atomic",
					statement,
					"do not assign an atomic operation result back to the same value",
				)
			}
			if unit := epochUnit(name); unit != "" {
				if id := rootIdent(statement.Lhs[0]); id != nil && !validEpochName(id.Name, unit) {
					a.report(
						"epoch-naming",
						id,
						fmt.Sprintf("name should end with a %s unit suffix", unit),
					)
				}
			}
		}
	}
	if statement.Tok == token.DEFINE {
		for _, lhs := range statement.Lhs {
			if id, ok := lhs.(*ast.Ident); ok {
				a.checkIdentifierName(id)
			}
		}
	}
}

func isOne(expr ast.Expr) bool {
	literal, ok := expr.(*ast.BasicLit)
	return ok && literal.Kind == token.INT && literal.Value == "1"
}

func epochUnit(name string) string {
	switch {
	case strings.HasSuffix(name, ".UnixNano"):
		return "nanosecond"
	case strings.HasSuffix(name, ".UnixMicro"):
		return "microsecond"
	case strings.HasSuffix(name, ".UnixMilli"):
		return "millisecond"
	case strings.HasSuffix(name, ".Unix"):
		return "second"
	}
	return ""
}

func validEpochName(name, unit string) bool {
	lower := strings.ToLower(name)
	var suffixes []string
	switch unit {
	case "second":
		suffixes = []string{"sec", "second", "seconds"}
	case "millisecond":
		suffixes = []string{"milli", "ms"}
	case "microsecond":
		suffixes = []string{"micro", "microsecond", "microseconds", "us"}
	case "nanosecond":
		suffixes = []string{"nano", "ns"}
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}
