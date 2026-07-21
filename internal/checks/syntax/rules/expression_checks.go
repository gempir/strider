package rules

import (
	"bytes"
	"go/constant"
	"go/token"

	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/diagnostic"
)

func (a *Pass) checkBinaryExpression(binary *cst.BinaryExpression) {
	if moduloOne(binary) {
		a.report("modulo-one", binary, "remainder modulo one is always zero")
	}
	if zeroIntegerLiteralDivision(binary) {
		a.report("zero-integer-division", binary, "integer literal division truncates to zero")
	}
	if binary.Op.Ch() == token.EQL || binary.Op.Ch() == token.NEQ {
		if booleanLiteral(binary.LHS) || booleanLiteral(binary.RHS) {
			a.report("boolean-literal-comparison", binary, "omit the boolean literal from the logical expression")
		}
	}
	if binary.Op.Ch() == token.LAND || binary.Op.Ch() == token.LOR {
		if value, ok := staticBool(binary.LHS); ok {
			if (binary.Op.Ch() == token.LAND && !value) || (binary.Op.Ch() == token.LOR && value) {
				a.report("constant-logical-expr", binary, "logical expression always has the same value")
			}
		}
	}
}

func staticBool(node cst.Node) (bool, bool) {
	spelling := cst.Spelling(unparen(node))
	if spelling == "true" {
		return true, true
	}
	if spelling == "false" {
		return false, true
	}
	return false, false
}

func moduloOne(binary *cst.BinaryExpression) bool {
	if binary.Op.Ch() != token.REM {
		return false
	}
	literal, ok := unparen(binary.RHS).(*cst.BasicLit)
	if !ok || literal.Ch() != token.INT {
		return false
	}
	value := constant.MakeFromLiteral(literal.Src(), token.INT, 0)
	return value.Kind() != constant.Unknown && constant.Compare(value, token.EQL, constant.MakeInt64(1))
}

func zeroIntegerLiteralDivision(binary *cst.BinaryExpression) bool {
	if binary.Op.Ch() != token.QUO {
		return false
	}
	left, leftOK := unparen(binary.LHS).(*cst.BasicLit)
	right, rightOK := unparen(binary.RHS).(*cst.BasicLit)
	if !leftOK || !rightOK || left.Ch() != token.INT || right.Ch() != token.INT {
		return false
	}
	numerator := constant.MakeFromLiteral(left.Src(), token.INT, 0)
	denominator := constant.MakeFromLiteral(right.Src(), token.INT, 0)
	if numerator.Kind() == constant.Unknown || denominator.Kind() == constant.Unknown || constant.Sign(denominator) == 0 {
		return false
	}
	return constant.Compare(numerator, token.LSS, denominator)
}

func unparen(node cst.Node) cst.Node {
	for {
		parenthesized, ok := node.(*cst.ParenthesizedExpr)
		if !ok {
			return node
		}
		node = parenthesized.Expression
	}
}

func booleanLiteral(node cst.Node) bool {
	spelling := cst.Spelling(unparen(node))
	return spelling == "true" || spelling == "false"
}

func (a *Pass) checkUnaryExpression(expression *cst.UnaryExpr) {
	inner, ok := expression.UnaryExpr.(*cst.UnaryExpr)
	if !ok {
		return
	}
	switch {
	case expression.Op.Ch() == token.NOT && inner.Op.Ch() == token.NOT:
		message := "double boolean negation has no effect and should be removed"
		outerStart := expression.Op.Position().Offset
		outerEnd := outerStart + len(expression.Op.Src())
		innerStart := inner.Op.Position().Offset
		innerEnd := innerStart + len(inner.Op.Src())
		operandStart, _ := cst.Range(inner.UnaryExpr)
		unsafeTrivia := automaticFixWouldJoinToken(a.content, outerStart) || unsafeAutomaticFixTrivia(a.content, outerEnd, innerStart) || unsafeAutomaticFixTrivia(
			a.content,
			innerEnd,
			operandStart,
		)
		if a.hasNegatingParent(expression) || unsafeTrivia {
			a.report("double-negation", expression, message)
			return
		}
		a.reportFix(
			"double-negation",
			expression,
			message,
			diagnostic.Fix{
				Message:   "remove the double negation",
				Safety:    diagnostic.Safe,
				Automatic: true,
				Edits: []diagnostic.TextEdit{
					{
						Start:   expression.Op.Position().Offset,
						End:     expression.Op.Position().Offset + len(expression.Op.Src()),
						OldText: expression.Op.Src(),
					},
					{
						Start:   inner.Op.Position().Offset,
						End:     inner.Op.Position().Offset + len(inner.Op.Src()),
						OldText: inner.Op.Src(),
					},
				},
			},
		)
	case expression.Op.Ch() == token.AND && inner.Op.Ch() == token.MUL && !cgoPointerIdentifier.MatchString(cst.Spelling(inner.UnaryExpr)):
		a.report("ineffective-pointer-copy", expression, "&*value simplifies to value and does not copy the pointed-to data")
	case expression.Op.Ch() == token.MUL && inner.Op.Ch() == token.AND:
		a.report("ineffective-pointer-copy", expression, "*&value simplifies to value and does not make a copy")
	}
}

func automaticFixWouldJoinToken(content []byte, offset int) bool {
	if offset <= 0 || offset > len(content) {
		return false
	}
	previous := content[offset-1]
	return previous == '_' || previous >= 0x80 || previous >= 'a' && previous <= 'z' || previous >= 'A' && previous <= 'Z' || previous >= '0' && previous <= '9'
}

func unsafeAutomaticFixTrivia(content []byte, start, end int) bool {
	if start < 0 || end < start || end > len(content) {
		return true
	}
	trivia := content[start:end]
	return bytes.ContainsAny(trivia, "\r\n") || bytes.Contains(trivia, []byte("//")) || bytes.Contains(trivia, []byte("/*"))
}

func (a *Pass) hasNegatingParent(expression *cst.UnaryExpr) bool {
	if len(a.ancestors) == 0 {
		return false
	}
	parent, ok := a.ancestors[len(a.ancestors)-1].(*cst.UnaryExpr)
	return ok && parent.Op.Ch() == token.NOT && parent.UnaryExpr == expression
}

func (a *Pass) checkInterfaceType(iface *cst.InterfaceType) {
	if iface.InterfaceElemList == nil {
		a.report("use-any", iface, "use any instead of interface{}")
	}
}

func (a *Pass) checkIncrementAssignment(statement *cst.Assignment) {
	if statement.Op.Ch() != token.ASSIGN || statement.ExpressionList == nil || statement.ExpressionList2 == nil || statement.ExpressionList.Len() != 1 || statement.ExpressionList2.Len() != 1 {
		return
	}
	a.checkIncrementParts(statement, statement.ExpressionList.Expression, statement.ExpressionList2.Expression)
}

func (a *Pass) checkIncrementShortDeclaration(statement *cst.ShortVarDecl) {
	if statement.IdentifierList == nil || statement.ExpressionList == nil || statement.IdentifierList.Len() != 1 || statement.ExpressionList.Len() != 1 {
		return
	}
	a.checkIncrementParts(statement, statement.IdentifierList, statement.ExpressionList.Expression)
}

func (a *Pass) checkIncrementParts(statement, left, right cst.Node) {
	binary, ok := right.(*cst.BinaryExpression)
	if !ok || (binary.Op.Ch() != token.ADD && binary.Op.Ch() != token.SUB) || cst.Spelling(left) != cst.Spelling(binary.LHS) {
		return
	}
	literal, ok := unparen(binary.RHS).(*cst.BasicLit)
	if !ok || literal.Ch() != token.INT || literal.Src() != "1" {
		return
	}
	a.report("increment-decrement", statement, "use ++ or -- instead of assigning an addition or subtraction of one")
}
