package rules

import (
	"fmt"
	"go/token"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

func (a *Pass) checkFunctionChecks(name cst.Token, signature *cst.Signature, body cst.Node, receiver *cst.Parameters, facts *functionFacts) {
	if signature == nil || facts == nil {
		return
	}
	parameters := parameterDecls(signature.Parameters)
	results := resultDecls(signature.Result)
	resultTotal := declCount(results)
	resultLimit := a.intOption("max-results")
	if a.active("function-result-limit") && resultTotal > resultLimit {
		a.report("function-result-limit", name, fmt.Sprintf("function returns %d values; maximum is %d", resultTotal, resultLimit))
	}
	if a.active("cognitive-complexity") && facts.cognitiveComplexity > 7 {
		a.report("cognitive-complexity", name, fmt.Sprintf("function has cognitive complexity %d; maximum is 7", facts.cognitiveComplexity))
	}
	if body != nil && (a.active("function-length") || a.active("redundant-final-return")) {
		a.checkFunctionBody(name, body, resultTotal, facts)
	}
	if a.active("get-function-return-value") && strings.HasPrefix(strings.ToUpper(name.Src()), "GET") && resultTotal == 0 {
		a.report("get-function-return-value", name, "Get-prefixed function should return a value")
	}
	if a.active("error-last-result") || a.active("unexported-return") || a.active("confusing-results") {
		a.checkResults(name, results)
	}
	if a.active("context-as-argument") || a.active("waitgroup-by-value") || a.active("time-naming") {
		a.checkParameters(parameters)
	}
	if a.active("unused-parameter") || a.active("flag-parameter") || a.active("unused-receiver") || a.active("receiver-naming") {
		a.checkUnused(parameters, receiver, body)
	}
	if receiver != nil && (a.active("receiver-naming") || a.active("marshal-receiver")) {
		a.checkReceiver(name, receiver)
	}
}

func (a *Pass) checkFunctionBody(name cst.Token, body cst.Node, resultTotal int, facts *functionFacts) {
	if a.active("function-length") {
		start, end := cst.Range(body)
		lines := a.tree.Position(end).Line - a.tree.Position(start).Line + 1
		statementLimit := a.intOption("max-statements")
		lineLimit := a.intOption("max-lines")
		if facts.statements > statementLimit || lines > lineLimit {
			a.report(
				"function-length",
				name,
				fmt.Sprintf("function has %d statements and %d lines; maximum is %d statements or %d lines", facts.statements, lines, statementLimit, lineLimit),
			)
		}
	}
	if resultTotal != 0 || !a.active("redundant-final-return") {
		return
	}
	if returned, ok := facts.finalStatement.(*cst.ReturnStmt); ok && returned.ExpressionList == nil {
		a.report("redundant-final-return", returned, "omit the unnecessary return at the end of a resultless function")
	}
}

func parameterDecls(parameters *cst.Parameters) []*cst.ParameterDecl {
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

func resultDecls(result *cst.Result) []*cst.ParameterDecl {
	if result == nil {
		return nil
	}
	if result.Parameters != nil {
		return parameterDecls(result.Parameters)
	}
	if result.TypeNode != nil {
		return []*cst.ParameterDecl{
			{
				TypeNode: result.TypeNode,
			},
		}
	}
	return nil
}

func declCount(declarations []*cst.ParameterDecl) int {
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

func (a *Pass) checkResults(name cst.Token, results []*cst.ParameterDecl) {
	previous := ""
	for index, result := range results {
		typeName := cst.Spelling(result.TypeNode)
		if typeName == "error" && index != len(results)-1 {
			a.report("error-last-result", result.TypeNode, "error should be the last returned value")
		}
		if token.IsExported(name.Src()) {
			base := strings.TrimLeft(typeName, "*[]")
			if identifierName(base) && base != "error" && !token.IsExported(base) && !builtinType(base) {
				a.report("unexported-return", result.TypeNode, "exported function returns an unexported type")
			}
		}
		if result.IdentifierList == nil && previous == typeName {
			a.report("confusing-results", result.TypeNode, "adjacent unnamed results of the same type should be named")
		}
		previous = typeName
	}
}

func (a *Pass) checkParameters(parameters []*cst.ParameterDecl) {
	position := 0
	for _, parameter := range parameters {
		typeName := cst.Spelling(parameter.TypeNode)
		if typeName == "context.Context" && position != 0 {
			a.report("context-as-argument", parameter.TypeNode, "context.Context should be the first parameter")
		}
		if typeName == "sync.WaitGroup" {
			a.report("waitgroup-by-value", parameter.TypeNode, "sync.WaitGroup should be passed by pointer")
		}
		names := identifierTokens(parameter.IdentifierList)
		for _, name := range names {
			if typeName == "time.Duration" && hasTimeUnitSuffix(name.Src()) {
				a.report("time-naming", name, "time.Duration name should not include a unit suffix")
			}
		}
		position += max(1, len(names))
	}
}

func (a *Pass) checkUnused(parameters []*cst.ParameterDecl, receiver *cst.Parameters, body cst.Node) {
	if body == nil {
		return
	}
	uses := map[string]int{}
	if a.active("unused-parameter") || a.active("unused-receiver") {
		for _, current := range cst.NodeTokens(body) {
			if current.Ch() == token.IDENT {
				uses[current.Src()]++
			}
		}
	}
	if a.active("unused-parameter") || a.active("flag-parameter") {
		for _, parameter := range parameters {
			for _, name := range identifierTokens(parameter.IdentifierList) {
				if a.active("unused-parameter") && name.Src() != "_" && uses[name.Src()] == 0 {
					a.report("unused-parameter", name, fmt.Sprintf("parameter %s is unused", name.Src()))
				}
				if a.active("flag-parameter") && cst.Spelling(parameter.TypeNode) == "bool" && conditionUses(body, name.Src()) {
					a.report("flag-parameter", name, fmt.Sprintf("boolean parameter %s controls function flow", name.Src()))
				}
			}
		}
	}
	for _, declaration := range parameterDecls(receiver) {
		for _, name := range identifierTokens(declaration.IdentifierList) {
			if a.active("unused-receiver") && name.Src() != "_" && uses[name.Src()] == 0 {
				a.report("unused-receiver", name, fmt.Sprintf("receiver %s is unused", name.Src()))
			}
			if a.active("receiver-naming") && (name.Src() == "this" || name.Src() == "self") {
				a.report("receiver-naming", name, "receiver name should be a short abbreviation of its type")
			}
		}
	}
}

func conditionUses(body cst.Node, name string) bool {
	found := false
	cst.Walk(
		body,
		func(node cst.Node) bool {
			var condition cst.Node
			switch current := node.(type) {
			case *cst.IfStmt:
				condition = current.Expression
			case *cst.IfElseStmt:
				condition = current.Expression
			default:
				return true
			}
			for _, current := range cst.NodeTokens(condition) {
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

func identifierTokens(list *cst.IdentifierList) []cst.Token {
	result := []cst.Token{}
	for ; list != nil; list = list.List {
		if list.IDENT.IsValid() {
			result = append(result, list.IDENT)
		}
	}
	return result
}

func (a *Pass) checkReceiver(name cst.Token, receiver *cst.Parameters) {
	declarations := parameterDecls(receiver)
	if len(declarations) == 0 {
		return
	}
	declaration := declarations[0]
	typeName := cst.Spelling(declaration.TypeNode)
	base := strings.TrimPrefix(typeName, "*")
	names := identifierTokens(declaration.IdentifierList)
	state := a.functionState()
	if a.active("receiver-naming") && len(names) != 0 {
		receiverName := names[0].Src()
		if first, ok := state.receiverNames[base]; ok && first != receiverName {
			a.report("receiver-naming", names[0], fmt.Sprintf("receiver name %s is inconsistent with %s", receiverName, first))
		} else {
			state.receiverNames[base] = receiverName
		}
	}
	if !a.active("marshal-receiver") || !marshalMethod(name.Src()) {
		return
	}
	kind := "value"
	if strings.HasPrefix(typeName, "*") {
		kind = "pointer"
	}
	if first, ok := state.marshalKinds[base]; ok && first != kind {
		a.report("marshal-receiver", declaration, "marshal and unmarshal methods should use a consistent receiver type")
	} else {
		state.marshalKinds[base] = kind
	}
}
