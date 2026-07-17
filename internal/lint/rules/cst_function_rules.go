package rules

import (
	"fmt"
	"go/token"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

func (a *cstAnalyzer) checkConcreteFunctionRules(
	name cst.Token,
	signature *cst.Signature,
	body cst.Node,
	receiver *cst.Parameters,
) {
	if signature == nil {
		return
	}
	parameters := concreteParameterDecls(signature.Parameters)
	results := concreteResultDecls(signature.Result)
	parameterTotal := concreteDeclCount(parameters)
	if parameterTotal > 8 {
		a.report(
			"argument-limit",
			name,
			fmt.Sprintf("function has %d parameters; maximum is 8", parameterTotal),
		)
	}
	resultTotal := concreteDeclCount(results)
	if resultTotal > 3 {
		a.report(
			"function-result-limit",
			name,
			fmt.Sprintf("function returns %d values; maximum is 3", resultTotal),
		)
	}
	complexity := cyclomaticComplexity(body)
	if complexity > 10 {
		a.report(
			"cyclomatic",
			name,
			fmt.Sprintf("function has cyclomatic complexity %d; maximum is 10", complexity),
		)
	}
	cognitive := concreteCognitiveComplexity(body)
	if cognitive > 7 {
		a.report(
			"cognitive-complexity",
			name,
			fmt.Sprintf("function has cognitive complexity %d; maximum is 7", cognitive),
		)
	}
	if body != nil {
		a.checkConcreteFunctionBody(name, body, resultTotal)
	}
	if strings.HasPrefix(strings.ToUpper(name.Src()), "GET") && resultTotal == 0 {
		a.report("get-return", name, "Get-prefixed function should return a value")
	}
	a.checkConcreteResults(name, results)
	a.checkConcreteParameters(parameters)
	a.checkConcreteUnused(parameters, receiver, body)
	if receiver != nil {
		a.checkConcreteReceiver(name, receiver)
	}
}

func (a *cstAnalyzer) checkConcreteFunctionBody(name cst.Token, body cst.Node, resultTotal int) {
	statements := concreteStatementCount(body)
	start, end := cst.Range(body)
	lines := a.tree.Position(end).Line - a.tree.Position(start).Line + 1
	if statements > 50 || lines > 75 {
		a.report(
			"function-length",
			name,
			fmt.Sprintf(
				"function has %d statements and %d lines; maximum is 50 statements or 75 lines",
				statements,
				lines,
			),
		)
	}
	if resultTotal != 0 {
		return
	}
	if returned, ok := concreteFinalStatement(body).(*cst.ReturnStmt); ok && returned.ExpressionList == nil {
		a.report(
			"unnecessary-stmt",
			returned,
			"omit the unnecessary return at the end of a resultless function",
		)
	}
}

func concreteParameterDecls(parameters *cst.Parameters) []*cst.ParameterDecl {
	result := []*cst.ParameterDecl{}
	if parameters == nil {
		return result
	}
	for list := parameters.ParameterDeclList; list != nil; list = list.List {
		if list.ParameterDecl != nil {
			result = append(result, list.ParameterDecl)
		}
	}
	return result
}

func concreteResultDecls(result *cst.Result) []*cst.ParameterDecl {
	if result == nil {
		return nil
	}
	if result.Parameters != nil {
		return concreteParameterDecls(result.Parameters)
	}
	if result.TypeNode != nil {
		return[]*cst.ParameterDecl{{TypeNode: result.TypeNode}}
	}
	return nil
}

func concreteDeclCount(declarations []*cst.ParameterDecl) int {
	count := 0
	for _, declaration := range declarations {
		if declaration.IdentifierList == nil {
			count++
		} else {
			count += declaration.IdentifierList.Len()
		}
	}
	return count
}

func concreteCognitiveComplexity(body cst.Node) int {
	if body == nil {
		return 0
	}
	total := 0
	var visit func(cst.Node, int)
	visit = func(node cst.Node, nesting int) {
		next := nesting
		switch cst.Kind(node) {
		case "IfStmt", "IfElseStmt", "ForStmt", "ExprSwitchStmt", "TypeSwitchStmt", "SelectStmt":
			total += 1 + nesting
			next++
		case "BreakStmt", "ContinueStmt", "GotoStmt", "FallthroughStmt":
			total++
		}
		for _, child := range cst.Children(node) {
			visit(child, next)
		}
	}
	visit(body, 0)
	return total
}

func concreteStatementCount(body cst.Node) int {
	count := 0
	cst.Walk(
		body,
		func(node cst.Node) bool {
			if list,
			ok := node.(*cst.StatementList); ok && list.Statement != nil {
				count++
			}
			return true
		},
	)
	return count
}

func concreteFinalStatement(body cst.Node) cst.Node {
	var block *cst.Block
	cst.Walk(
		body,
		func(node cst.Node) bool {
			if current,
			ok := node.(*cst.Block); ok && block == nil {
				block = current
				return false
			}
			return block == nil
		},
	)
	if block == nil {
		return nil
	}
	var final cst.Node
	for list := block.StatementList; list != nil; list = list.List {
		if list.Statement != nil {
			final = list.Statement
		}
	}
	return final
}

func (a *cstAnalyzer) checkConcreteResults(name cst.Token, results []*cst.ParameterDecl) {
	previous := ""
	for index, result := range results {
		typeName := cst.Spelling(result.TypeNode)
		if typeName == "error" && index != len(results) - 1 {
			a.report("error-return", result.TypeNode, "error should be the last returned value")
		}
		if token.IsExported(name.Src()) {
			base := strings.TrimLeft(typeName, "*[]")
			if identifierName(base) && base != "error" && !token.IsExported(base) && !builtinType(
				base,
			) {
				a.report(
					"unexported-return",
					result.TypeNode,
					"exported function returns an unexported type",
				)
			}
		}
		if result.IdentifierList == nil && previous == typeName {
			a.report(
				"confusing-results",
				result.TypeNode,
				"adjacent unnamed results of the same type should be named",
			)
		}
		previous = typeName
	}
}

func (a *cstAnalyzer) checkConcreteParameters(parameters []*cst.ParameterDecl) {
	position := 0
	for _, parameter := range parameters {
		typeName := cst.Spelling(parameter.TypeNode)
		if typeName == "context.Context" && position != 0 {
			a.report(
				"context-as-argument",
				parameter.TypeNode,
				"context.Context should be the first parameter",
			)
		}
		if typeName == "sync.WaitGroup" {
			a.report(
				"waitgroup-by-value",
				parameter.TypeNode,
				"sync.WaitGroup should be passed by pointer",
			)
		}
		names := concreteIdentifierTokens(parameter.IdentifierList)
		for _, name := range names {
			if typeName == "time.Duration" && hasTimeUnitSuffix(name.Src()) {
				a.report(
					"time-naming",
					name,
					"time.Duration name should not include a unit suffix",
				)
			}
		}
		position += max(1, len(names))
	}
}

func (a *cstAnalyzer) checkConcreteUnused(
	parameters []*cst.ParameterDecl,
	receiver *cst.Parameters,
	body cst.Node,
) {
	if body == nil {
		return
	}
	uses := map[string]int{}
	for _, current := range cst.NodeTokens(body) {
		if current.Ch() == token.IDENT {
			uses[current.Src()]++
		}
	}
	for _, parameter := range parameters {
		for _, name := range concreteIdentifierTokens(parameter.IdentifierList) {
			if name.Src() != "_" && uses[name.Src()] == 0 {
				a.report(
					"unused-parameter",
					name,
					fmt.Sprintf("parameter %s is unused", name.Src()),
				)
			}
			if cst.Spelling(parameter.TypeNode) == "bool" && concreteConditionUses(body, name.Src()) {
				a.report(
					"flag-parameter",
					name,
					fmt.Sprintf("boolean parameter %s controls function flow", name.Src()),
				)
			}
		}
	}
	for _, declaration := range concreteParameterDecls(receiver) {
		for _, name := range concreteIdentifierTokens(declaration.IdentifierList) {
			if name.Src() != "_" && uses[name.Src()] == 0 {
				a.report(
					"unused-receiver",
					name,
					fmt.Sprintf("receiver %s is unused", name.Src()),
				)
			}
			if name.Src() == "this" || name.Src() == "self" {
				a.report(
					"receiver-naming",
					name,
					"receiver name should be a short abbreviation of its type",
				)
			}
		}
	}
}

func concreteConditionUses(body cst.Node, name string) bool {
	found := false
	cst.Walk(
		body,
		func(node cst.Node) bool {
			kind := cst.Kind(node)
			if kind != "IfStmt" && kind != "IfElseStmt" {
				return true
			}
			for _,
			current := range cst.NodeTokens(node) {
				if current.Ch() == token.IDENT && current.Src() == name {
					found = true
					return false
				}
			}
			return !found
		},
	)
	return found
}

func concreteIdentifierTokens(list *cst.IdentifierList) []cst.Token {
	result := []cst.Token{}
	for; list != nil; list = list.List {
		if list.IDENT.IsValid() {
			result = append(result, list.IDENT)
		}
	}
	return result
}

func (a *cstAnalyzer) checkConcreteReceiver(name cst.Token, receiver *cst.Parameters) {
	declarations := concreteParameterDecls(receiver)
	if len(declarations) == 0 {
		return
	}
	declaration := declarations[0]
	typeName := cst.Spelling(declaration.TypeNode)
	base := strings.TrimPrefix(typeName, "*")
	names := concreteIdentifierTokens(declaration.IdentifierList)
	if len(names) != 0 {
		receiverName := names[0].Src()
		if first, ok := a.receiverNames[base]; ok && first != receiverName {
			a.report(
				"receiver-naming",
				names[0],
				fmt.Sprintf("receiver name %s is inconsistent with %s", receiverName, first),
			)
		} else {
			a.receiverNames[base] = receiverName
		}
	}
	if !marshalMethod(name.Src()) {
		return
	}
	kind := "value"
	if strings.HasPrefix(typeName, "*") {
		kind = "pointer"
	}
	if first, ok := a.marshalKinds[base]; ok && first != kind {
		a.report(
			"marshal-receiver",
			declaration,
			"marshal and unmarshal methods should use a consistent receiver type",
		)
	} else {
		a.marshalKinds[base] = kind
	}
}
