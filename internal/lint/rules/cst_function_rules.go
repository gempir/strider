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
	facts *cstFunctionFacts,
) {
	if signature == nil || facts == nil {
		return
	}
	parameters := concreteParameterDecls(signature.Parameters)
	results := concreteResultDecls(signature.Result)
	parameterTotal := concreteDeclCount(parameters)
	if a.enabled["argument-limit"] && parameterTotal > 8 {
		a.report(
			"argument-limit",
			name,
			fmt.Sprintf("function has %d parameters; maximum is 8", parameterTotal),
		)
	}
	resultTotal := concreteDeclCount(results)
	if a.enabled["function-result-limit"] && resultTotal > 3 {
		a.report(
			"function-result-limit",
			name,
			fmt.Sprintf("function returns %d values; maximum is 3", resultTotal),
		)
	}
	if a.enabled["cyclomatic"] && facts.complexity > 10 {
		a.report(
			"cyclomatic",
			name,
			fmt.Sprintf("function has cyclomatic complexity %d; maximum is 10", facts.complexity),
		)
	}
	if a.enabled["cognitive-complexity"] && facts.cognitiveComplexity > 7 {
		a.report(
			"cognitive-complexity",
			name,
			fmt.Sprintf(
				"function has cognitive complexity %d; maximum is 7",
				facts.cognitiveComplexity,
			),
		)
	}
	if body != nil && (a.enabled["function-length"] || a.enabled["unnecessary-stmt"]) {
		a.checkConcreteFunctionBody(name, body, resultTotal, facts)
	}
	if a.enabled["get-return"] && strings.HasPrefix(strings.ToUpper(name.Src()), "GET") && resultTotal == 0 {
		a.report("get-return", name, "Get-prefixed function should return a value")
	}
	if a.enabled["error-return"] || a.enabled["unexported-return"] || a.enabled["confusing-results"] {
		a.checkConcreteResults(name, results)
	}
	if a.enabled["context-as-argument"] || a.enabled["waitgroup-by-value"] || a.enabled["time-naming"] {
		a.checkConcreteParameters(parameters)
	}
	if a.enabled["unused-parameter"] || a.enabled["flag-parameter"] || a.enabled["unused-receiver"] || a.enabled["receiver-naming"] {
		a.checkConcreteUnused(parameters, receiver, body)
	}
	if receiver != nil && (a.enabled["receiver-naming"] || a.enabled["marshal-receiver"]) {
		a.checkConcreteReceiver(name, receiver)
	}
}

func (a *cstAnalyzer) checkConcreteFunctionBody(
	name cst.Token,
	body cst.Node,
	resultTotal int,
	facts *cstFunctionFacts,
) {
	if a.enabled["function-length"] {
		start, end := cst.Range(body)
		lines := a.tree.Position(end).Line - a.tree.Position(start).Line + 1
		if facts.statements > 50 || lines > 75 {
			a.report(
				"function-length",
				name,
				fmt.Sprintf(
					"function has %d statements and %d lines; maximum is 50 statements or 75 lines",
					facts.statements,
					lines,
				),
			)
		}
	}
	if resultTotal != 0 || !a.enabled["unnecessary-stmt"] {
		return
	}
	if returned, ok := facts.finalStatement.(*cst.ReturnStmt); ok && returned.ExpressionList == nil {
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
	if a.enabled["unused-parameter"] || a.enabled["unused-receiver"] {
		for _, current := range cst.NodeTokens(body) {
			if current.Ch() == token.IDENT {
				uses[current.Src()]++
			}
		}
	}
	if a.enabled["unused-parameter"] || a.enabled["flag-parameter"] {
		for _, parameter := range parameters {
			for _, name := range concreteIdentifierTokens(parameter.IdentifierList) {
				if a.enabled["unused-parameter"] && name.Src() != "_" && uses[name.Src()] == 0 {
					a.report(
						"unused-parameter",
						name,
						fmt.Sprintf("parameter %s is unused", name.Src()),
					)
				}
				if a.enabled["flag-parameter"] && cst.Spelling(parameter.TypeNode) == "bool" && concreteConditionUses(
					body,
					name.Src(),
				) {
					a.report(
						"flag-parameter",
						name,
						fmt.Sprintf("boolean parameter %s controls function flow", name.Src()),
					)
				}
			}
		}
	}
	for _, declaration := range concreteParameterDecls(receiver) {
		for _, name := range concreteIdentifierTokens(declaration.IdentifierList) {
			if a.enabled["unused-receiver"] && name.Src() != "_" && uses[name.Src()] == 0 {
				a.report(
					"unused-receiver",
					name,
					fmt.Sprintf("receiver %s is unused", name.Src()),
				)
			}
			if a.enabled["receiver-naming"] && (name.Src() == "this" || name.Src() == "self") {
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
	if a.enabled["receiver-naming"] && len(names) != 0 {
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
	if !a.enabled["marshal-receiver"] || !marshalMethod(name.Src()) {
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
