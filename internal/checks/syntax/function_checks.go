package syntax

import (
	"fmt"
	"go/token"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

func (a *Pass) checkCyclomaticComplexity(facts *functionFacts) {
	if facts.signature != nil && facts.complexity > 10 {
		a.Report(facts.name, fmt.Sprintf("function complexity is %d; maximum is 10", facts.complexity))
	}
}

func (a *Pass) checkMaxParameters(facts *functionFacts) {
	if facts.signature == nil {
		return
	}
	count := parameterCount(facts.signature.Parameters)
	limit := a.intOption("max-parameters")
	if count > limit {
		a.Report(facts.name, fmt.Sprintf("function has %d parameters; maximum is %d", count, limit))
	}
}

func (a *Pass) checkCognitiveComplexity(facts *functionFacts) {
	if facts.signature != nil && facts.cognitiveComplexity > 7 {
		a.Report(facts.name, fmt.Sprintf("function has cognitive complexity %d; maximum is 7", facts.cognitiveComplexity))
	}
}

func (a *Pass) checkFunctionResultLimit(facts *functionFacts) {
	if facts.signature == nil {
		return
	}
	results := resultDecls(facts.signature.Result)
	resultTotal := declCount(results)
	resultLimit := a.intOption("max-results")
	if resultTotal > resultLimit {
		a.Report(facts.name, fmt.Sprintf("function returns %d values; maximum is %d", resultTotal, resultLimit))
	}
}

func (a *Pass) checkFunctionLength(facts *functionFacts) {
	if facts.signature == nil || facts.body == nil {
		return
	}
	start, end := cst.Range(facts.body)
	lines := a.tree.Position(end).Line - a.tree.Position(start).Line + 1
	statementLimit := a.intOption("max-statements")
	lineLimit := a.intOption("max-lines")
	if facts.statements > statementLimit || lines > lineLimit {
		a.Report(
			facts.name,
			fmt.Sprintf("function has %d statements and %d lines; maximum is %d statements or %d lines", facts.statements, lines, statementLimit, lineLimit),
		)
	}
}

func (a *Pass) checkRedundantFinalReturn(facts *functionFacts) {
	if facts.signature == nil || declCount(resultDecls(facts.signature.Result)) != 0 {
		return
	}
	if returned, ok := facts.finalStatement.(*cst.ReturnStmt); ok && returned.ExpressionList == nil {
		a.Report(returned, "omit the unnecessary return at the end of a resultless function")
	}
}

func (a *Pass) checkGetFunctionReturnValue(facts *functionFacts) {
	if facts.signature == nil {
		return
	}
	if strings.HasPrefix(strings.ToUpper(facts.name.Src()), "GET") && declCount(resultDecls(facts.signature.Result)) == 0 {
		a.Report(facts.name, "Get-prefixed function should return a value")
	}
}

func (a *Pass) checkModifiesParameter(facts *functionFacts) {
	if facts.signature != nil {
		a.checkParameterMutation(facts.signature.Parameters, facts.body)
	}
}

func (a *Pass) checkModifiesValueReceiver(facts *functionFacts) {
	if facts.signature != nil && facts.receiver != nil {
		a.checkValueReceiverMutation(facts.receiver, facts.body)
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

func (a *Pass) checkErrorLastResult(facts *functionFacts) {
	if facts.signature == nil {
		return
	}
	results := resultDecls(facts.signature.Result)
	for index, result := range results {
		if cst.Spelling(result.TypeNode) == "error" && index != len(results)-1 {
			a.Report(result.TypeNode, "error should be the last returned value")
		}
	}
}

func (a *Pass) checkUnexportedReturn(facts *functionFacts) {
	if facts.signature == nil || !token.IsExported(facts.name.Src()) {
		return
	}
	for _, result := range resultDecls(facts.signature.Result) {
		typeName := cst.Spelling(result.TypeNode)
		base := strings.TrimLeft(typeName, "*[]")
		if identifierName(base) && base != "error" && !token.IsExported(base) && !builtinType(base) {
			a.Report(result.TypeNode, "exported function returns an unexported type")
		}
	}
}

func (a *Pass) checkConfusingResults(facts *functionFacts) {
	if facts.signature == nil {
		return
	}
	previous := ""
	for _, result := range resultDecls(facts.signature.Result) {
		typeName := cst.Spelling(result.TypeNode)
		if result.IdentifierList == nil && previous == typeName {
			a.Report(result.TypeNode, "adjacent unnamed results of the same type should be named")
		}
		previous = typeName
	}
}

func (a *Pass) checkContextAsArgument(facts *functionFacts) {
	if facts.signature == nil {
		return
	}
	position := 0
	for _, parameter := range parameterDecls(facts.signature.Parameters) {
		typeName := cst.Spelling(parameter.TypeNode)
		if typeName == "context.Context" && position != 0 {
			a.Report(parameter.TypeNode, "context.Context should be the first parameter")
		}
		position += max(1, len(identifierTokens(parameter.IdentifierList)))
	}
}

func (a *Pass) checkWaitgroupByValue(facts *functionFacts) {
	if facts.signature == nil {
		return
	}
	for _, parameter := range parameterDecls(facts.signature.Parameters) {
		typeName := cst.Spelling(parameter.TypeNode)
		if typeName == "sync.WaitGroup" {
			a.Report(parameter.TypeNode, "sync.WaitGroup should be passed by pointer")
		}
	}
}

func (a *Pass) checkTimeNaming(facts *functionFacts) {
	if facts.signature == nil {
		return
	}
	for _, parameter := range parameterDecls(facts.signature.Parameters) {
		typeName := cst.Spelling(parameter.TypeNode)
		names := identifierTokens(parameter.IdentifierList)
		for _, name := range names {
			if typeName == "time.Duration" && hasTimeUnitSuffix(name.Src()) {
				a.Report(name, "time.Duration name should not include a unit suffix")
			}
		}
	}
}

func (a *Pass) functionUses(facts *functionFacts) map[string]int {
	uses := map[string]int{}
	if facts.body == nil {
		return uses
	}
	for _, current := range cst.NodeTokens(facts.body) {
		if current.Ch() == token.IDENT {
			uses[current.Src()]++
		}
	}
	return uses
}

func (a *Pass) checkUnusedParameter(facts *functionFacts) {
	if facts.signature == nil || facts.body == nil {
		return
	}
	uses := a.functionUses(facts)
	for _, parameter := range parameterDecls(facts.signature.Parameters) {
		for _, name := range identifierTokens(parameter.IdentifierList) {
			if name.Src() != "_" && uses[name.Src()] == 0 {
				a.Report(name, fmt.Sprintf("parameter %s is unused", name.Src()))
			}
		}
	}
}

func (a *Pass) checkFlagParameter(facts *functionFacts) {
	if facts.signature == nil || facts.body == nil {
		return
	}
	for _, parameter := range parameterDecls(facts.signature.Parameters) {
		for _, name := range identifierTokens(parameter.IdentifierList) {
			if cst.Spelling(parameter.TypeNode) == "bool" && conditionUses(facts.body, name.Src()) {
				a.Report(name, fmt.Sprintf("boolean parameter %s controls function flow", name.Src()))
			}
		}
	}
}

func (a *Pass) checkUnusedReceiver(facts *functionFacts) {
	if facts.signature == nil || facts.body == nil {
		return
	}
	uses := a.functionUses(facts)
	for _, declaration := range parameterDecls(facts.receiver) {
		for _, name := range identifierTokens(declaration.IdentifierList) {
			if name.Src() != "_" && uses[name.Src()] == 0 {
				a.Report(name, fmt.Sprintf("receiver %s is unused", name.Src()))
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

func (a *Pass) checkReceiverNaming(facts *functionFacts) {
	declarations := parameterDecls(facts.receiver)
	if len(declarations) == 0 {
		return
	}
	declaration := declarations[0]
	typeName := cst.Spelling(declaration.TypeNode)
	base := strings.TrimPrefix(typeName, "*")
	names := identifierTokens(declaration.IdentifierList)
	state := a.functionState()
	if len(names) != 0 {
		receiverName := names[0].Src()
		if first, ok := state.receiverNames[base]; ok && first != receiverName {
			a.Report(names[0], fmt.Sprintf("receiver name %s is inconsistent with %s", receiverName, first))
		} else {
			state.receiverNames[base] = receiverName
		}
	}
	if facts.body != nil {
		for _, name := range names {
			if name.Src() == "this" || name.Src() == "self" {
				a.Report(name, "receiver name should be a short abbreviation of its type")
			}
		}
	}
}

func (a *Pass) checkMarshalReceiver(facts *functionFacts) {
	if !marshalMethod(facts.name.Src()) {
		return
	}
	declarations := parameterDecls(facts.receiver)
	if len(declarations) == 0 {
		return
	}
	declaration := declarations[0]
	typeName := cst.Spelling(declaration.TypeNode)
	base := strings.TrimPrefix(typeName, "*")
	state := a.functionState()
	kind := "value"
	if strings.HasPrefix(typeName, "*") {
		kind = "pointer"
	}
	if first, ok := state.marshalKinds[base]; ok && first != kind {
		a.Report(declaration, "marshal and unmarshal methods should use a consistent receiver type")
	} else {
		state.marshalKinds[base] = kind
	}
}
