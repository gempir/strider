package syntax

import (
	"fmt"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

func (a *Pass) checkAssignmentPolicy(statement *cst.Assignment) {
	if statement == nil || statement.ExpressionList == nil || statement.ExpressionList2 == nil || statement.ExpressionList.Len() != 1 || statement.ExpressionList2.Len() != 1 {
		return
	}
	a.checkAssignedCall(statement, statement.ExpressionList.Expression, statement.ExpressionList2.Expression)
}

func (a *Pass) checkShortDeclarationPolicy(statement *cst.ShortVarDecl) {
	if statement == nil || statement.IdentifierList == nil || statement.ExpressionList == nil || statement.IdentifierList.Len() != 1 || statement.ExpressionList.Len() != 1 {
		return
	}
	a.checkAssignedCall(statement, statement.IdentifierList, statement.ExpressionList.Expression)
}

func (a *Pass) checkAssignedCall(statement, left, right cst.Node) {
	call, ok := right.(*cst.PrimaryExpr)
	if !ok {
		return
	}
	name := callName(call)
	if strings.HasPrefix(name, "atomic.") {
		arguments := callArguments(call.Postfix)
		if len(arguments) > 0 && strings.TrimPrefix(cst.Spelling(arguments[0]), "&") == cst.Spelling(left) {
			a.Report(statement, "do not assign an atomic operation result back to the same value")
		}
	}
}

func (a *Pass) checkParameterMutation(parameters *cst.Parameters, body cst.Node) {
	if body == nil {
		return
	}
	parameterSet := parameterNames(parameters)
	a.inspectMutationTargets(
		body,
		func(target cst.Node) {
			root := rootIdentifier(target)
			if root.IsValid() && parameterSet[root.Src()] {
				a.Report(target, fmt.Sprintf("assignment modifies parameter %s", root.Src()))
			}
		},
	)
}

func (a *Pass) checkValueReceiverMutation(receiver *cst.Parameters, body cst.Node) {
	if body == nil || receiver == nil || !valueReceiver(receiver) {
		return
	}
	receiverSet := parameterNames(receiver)
	a.inspectMutationTargets(
		body,
		func(target cst.Node) {
			root := rootIdentifier(target)
			if root.IsValid() && receiverSet[root.Src()] {
				a.Report(target, fmt.Sprintf("assignment modifies value receiver %s", root.Src()))
			}
		},
	)
}

func (a *Pass) inspectMutationTargets(body cst.Node, inspect func(cst.Node)) {
	cst.Walk(
		body,
		func(node cst.Node) bool {
			switch current := node.(type) {
			case *cst.Assignment:
				for list := current.ExpressionList; list != nil; list = list.List {
					inspect(list.Expression)
				}
			case *cst.IncDecStmt:
				inspect(current.Expression)
			}
			return true
		},
	)
}

func parameterNames(parameters *cst.Parameters) map[string]bool {
	result := map[string]bool{}
	for _, declaration := range parameterDecls(parameters) {
		for _, name := range identifierTokens(declaration.IdentifierList) {
			result[name.Src()] = true
		}
	}
	return result
}

func valueReceiver(receiver *cst.Parameters) bool {
	declarations := parameterDecls(receiver)
	return len(declarations) != 0 && !strings.HasPrefix(cst.Spelling(declarations[0].TypeNode), "*")
}

func rootIdentifier(node cst.Node) cst.Token {
	for _, current := range cst.NodeTokens(node) {
		if current.Ch().String() == "IDENT" {
			return current
		}
	}
	return cst.Token{}
}
