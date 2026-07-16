package rules

import (
	"fmt"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

func (a *cstAnalyzer) checkConcreteAssignmentPolicy(statement *cst.Assignment) {
	if statement == nil || statement.ExpressionList == nil || statement.ExpressionList2 == nil ||
		statement.ExpressionList.Len() != 1 || statement.ExpressionList2.Len() != 1 {
		return
	}
	a.checkConcreteAssignedCall(
		statement,
		statement.ExpressionList.Expression,
		statement.ExpressionList2.Expression,
	)
}

func (a *cstAnalyzer) checkConcreteShortDeclarationPolicy(statement *cst.ShortVarDecl) {
	if statement == nil || statement.IdentifierList == nil || statement.ExpressionList == nil ||
		statement.IdentifierList.Len() != 1 || statement.ExpressionList.Len() != 1 {
		return
	}
	a.checkConcreteAssignedCall(
		statement,
		statement.IdentifierList,
		statement.ExpressionList.Expression,
	)
}

func (a *cstAnalyzer) checkConcreteAssignedCall(statement, left, right cst.Node) {
	call, ok := right.(*cst.PrimaryExpr)
	if !ok {
		return
	}
	name := concreteCallName(call)
	root := concreteRootIdentifier(left)
	if strings.HasPrefix(name, "atomic.") {
		arguments := concreteCallArguments(call.Postfix)
		if len(arguments) > 0 && strings.TrimPrefix(cst.Spelling(arguments[0]), "&") ==
			cst.Spelling(left) {
			a.report(
				"atomic",
				statement,
				"do not assign an atomic operation result back to the same value",
			)
		}
	}
	if unit := concreteEpochUnit(name); unit != "" && root.IsValid() &&
		!validEpochName(root.Src(), unit) {
		a.report(
			"epoch-naming",
			root,
			fmt.Sprintf("name should end with a %s unit suffix", unit),
		)
	}
}

func concreteEpochUnit(name string) string {
	switch {
	case strings.HasSuffix(name, ".UnixNano"):
		return "nanosecond"
	case strings.HasSuffix(name, ".UnixMicro"):
		return "microsecond"
	case strings.HasSuffix(name, ".UnixMilli"):
		return "millisecond"
	case strings.HasSuffix(name, ".Unix"):
		return "second"
	default:
		return ""
	}
}

func (a *cstAnalyzer) checkConcreteFunctionMutation(
	parameters *cst.Parameters,
	receiver *cst.Parameters,
	body cst.Node,
) {
	if body == nil {
		return
	}
	parameterNames := concreteParameterNames(parameters)
	receiverNames := concreteParameterNames(receiver)
	valueReceiver := receiver != nil && concreteValueReceiver(receiver)
	cst.Walk(body, func(node cst.Node) bool {
		switch current := node.(type) {
		case *cst.Assignment:
			for list := current.ExpressionList; list != nil; list = list.List {
				a.reportConcreteMutation(list.Expression, parameterNames, receiverNames, valueReceiver)
			}
		case *cst.IncDecStmt:
			a.reportConcreteMutation(current.Expression, parameterNames, receiverNames, valueReceiver)
		}
		return true
	})
}

func concreteParameterNames(parameters *cst.Parameters) map[string]bool {
	result := map[string]bool{}
	for _, declaration := range concreteParameterDecls(parameters) {
		for _, name := range concreteIdentifierTokens(declaration.IdentifierList) {
			result[name.Src()] = true
		}
	}
	return result
}

func concreteValueReceiver(receiver *cst.Parameters) bool {
	declarations := concreteParameterDecls(receiver)
	return len(declarations) != 0 && !strings.HasPrefix(cst.Spelling(declarations[0].TypeNode), "*")
}

func (a *cstAnalyzer) reportConcreteMutation(
	target cst.Node,
	parameters, receivers map[string]bool,
	valueReceiver bool,
) {
	root := concreteRootIdentifier(target)
	if !root.IsValid() {
		return
	}
	if parameters[root.Src()] {
		a.report(
			"modifies-parameter",
			target,
			fmt.Sprintf("assignment modifies parameter %s", root.Src()),
		)
	}
	if valueReceiver && receivers[root.Src()] {
		a.report(
			"modifies-value-receiver",
			target,
			fmt.Sprintf("assignment modifies value receiver %s", root.Src()),
		)
	}
}

func concreteRootIdentifier(node cst.Node) cst.Token {
	for _, current := range cst.NodeTokens(node) {
		if current.Ch().String() == "IDENT" {
			return current
		}
	}
	return cst.Token{}
}
