package rules

import "go/ast"

func (a *analyzer) checkTypeAssertion(assertion *ast.TypeAssertExpr) {
	parent := a.parent()
	if assignment, ok := parent.(*ast.AssignStmt); ok && len(assignment.Lhs) == 2 {
		return
	}
	if _, ok := parent.(*ast.TypeSwitchStmt); ok {
		return
	}
	if assertion.Type != nil {
		a.report(
			"unchecked-type-assertion",
			assertion,
			"use the checked two-result form of the type assertion",
		)
	}
}
