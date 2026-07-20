package rules

import (
	"fmt"
	"go/token"

	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/diagnostic"
)

type Pass struct {
	filename         string
	tree             *cst.Tree
	content          []byte
	reporter         func(Finding)
	ancestors        []cst.Node
	current          cst.Node
	activeCode       string
	bannedCharacters map[rune]bool
	limits           map[string]int
	blockedImports   map[string]bool
	checkState       map[checkStateKey]any
}

// AnalyzeCST runs selected native CST checks over one lossless source tree.
func AnalyzeCST(input CSTInput) {
	if len(input.Checks) == 0 || input.Tree == nil {
		return
	}
	analyzer := &Pass{
		filename:         input.Filename,
		tree:             input.Tree,
		content:          input.Tree.Bytes(),
		reporter:         input.Report,
		limits:           input.Limits,
		bannedCharacters: make(map[rune]bool, len(input.BannedCharacters)),
		blockedImports:   make(map[string]bool, len(input.BlockedImports)),
		checkState:       make(map[checkStateKey]any),
	}
	for _, path := range input.BlockedImports {
		analyzer.blockedImports[path] = true
	}
	for _, character := range input.BannedCharacters {
		analyzer.bannedCharacters[character] = true
	}
	dispatch := make(map[NodeKind][]Check)
	for _, check := range input.Checks {
		for _, interest := range check.Interests() {
			dispatch[interest] = append(dispatch[interest], check)
		}
	}
	analyzer.dispatch(dispatch[fileNodeKind], nil)
	cst.WalkProductionsWithAncestors(
		input.Tree.Root(),
		func(node cst.Node, ancestors []cst.Node) bool {
			analyzer.ancestors = ancestors
			analyzer.current = node
			analyzer.dispatch(dispatch[NodeKind(cst.Kind(node))], node)
			return true
		},
	)
	analyzer.dispatch(dispatch[finishNodeKind], nil)
}

func (a *Pass) dispatch(checks []Check, node cst.Node) {
	for _, check := range checks {
		a.activeCode = check.Meta().Code
		check.Inspect(a, node)
	}
	a.activeCode = ""
}

func (a *Pass) active(code string) bool {
	return a.activeCode == code
}

func (a *Pass) report(code string, node cst.Node, message string) {
	if code != a.activeCode || node == nil || a.reporter == nil {
		return
	}
	a.reporter(Finding{
		Node:    node,
		Code:    code,
		Message: message,
	})
}

func (a *Pass) reportFix(code string, node cst.Node, message string, fix diagnostic.Fix) {
	if code != a.activeCode || node == nil || a.reporter == nil {
		return
	}
	a.reporter(Finding{
		Node:    node,
		Code:    code,
		Message: message,
		Fixes: []diagnostic.Fix{
			fix,
		},
	})
}

func (a *Pass) reportRange(code string, start, end int, message string) {
	if code != a.activeCode || a.reporter == nil {
		return
	}
	a.reporter(Finding{
		Start:    start,
		End:      end,
		HasRange: true,
		Code:     code,
		Message:  message,
	})
}

func (a *Pass) packageNameToken() cst.Token {
	tokens := a.tree.Tokens()
	for index, current := range tokens {
		if current.Ch() == token.PACKAGE && index+1 < len(tokens) {
			return tokens[index+1]
		}
	}
	return cst.Token{}
}

func (a *Pass) checkFunction(function *cst.FunctionDecl, facts *functionFacts) {
	if function.FunctionName == nil || function.Signature == nil {
		return
	}
	name := function.FunctionName.IDENT
	a.checkSignature(name, function.Signature.Parameters, facts.complexity)
	a.checkFunctionChecks(name, function.Signature, function.FunctionBody, nil, facts)
	if a.active("modifies-parameter") {
		a.checkFunctionMutation(function.Signature.Parameters, nil, function.FunctionBody)
	}
}

func (a *Pass) checkMethod(method *cst.MethodDecl, facts *functionFacts) {
	if method.Signature != nil {
		a.checkSignature(method.MethodName, method.Signature.Parameters, facts.complexity)
		a.checkFunctionChecks(method.MethodName, method.Signature, method.FunctionBody, method.Receiver, facts)
		if a.active("modifies-parameter") || a.active("modifies-value-receiver") {
			a.checkFunctionMutation(method.Signature.Parameters, method.Receiver, method.FunctionBody)
		}
	}
}

func (a *Pass) checkSignature(name cst.Token, parameters *cst.Parameters, complexity int) {
	if a.active("max-parameters") {
		count := parameterCount(parameters)
		limit := a.limit("max-parameters")
		if count > limit {
			a.report("max-parameters", name, fmt.Sprintf("function has %d parameters; maximum is %d", count, limit))
		}
	}
	if a.active("cyclomatic-complexity") {
		if complexity > 10 {
			a.report("cyclomatic-complexity", name, fmt.Sprintf("function complexity is %d; maximum is 10", complexity))
		}
	}
}

func (a *Pass) limit(code string) int {
	return a.limits[code]
}

func parameterCount(parameters *cst.Parameters) int {
	if parameters == nil {
		return 0
	}
	count := 0
	for list := parameters.ParameterDeclList; list != nil; list = list.List {
		declaration := list.ParameterDecl
		if declaration == nil || declaration.IdentifierList == nil {
			count++
			continue
		}
		count += declaration.IdentifierList.Len()
	}
	return count
}

func (a *Pass) checkNakedReturn(statement *cst.ReturnStmt) {
	if statement.ExpressionList != nil || !enclosingFunctionHasNamedResults(a.ancestors) {
		return
	}
	a.report("no-naked-return", statement, "return values must be explicit")
}

func enclosingFunctionHasNamedResults(ancestors []cst.Node) bool {
	for index := len(ancestors) - 1; index >= 0; index-- {
		var parameters *cst.Parameters
		switch function := ancestors[index].(type) {
		case *cst.FunctionDecl:
			if function.Signature != nil && function.Signature.Result != nil {
				parameters = function.Signature.Result.Parameters
			}
		case *cst.MethodDecl:
			if function.Signature != nil && function.Signature.Result != nil {
				parameters = function.Signature.Result.Parameters
			}
		case *cst.FunctionLit:
			if function.Signature != nil && function.Signature.Result != nil {
				parameters = function.Signature.Result.Parameters
			}
		default:
			continue
		}
		if parameters == nil {
			return false
		}
		for list := parameters.ParameterDeclList; list != nil; list = list.List {
			if list.ParameterDecl != nil && list.ParameterDecl.IdentifierList != nil {
				return true
			}
		}
		return false
	}
	return false
}

func (a *Pass) checkDefer(statement *cst.DeferStmt) {
	insideLoop := false
	for index := len(a.ancestors) - 1; index >= 0; index-- {
		switch a.ancestors[index].(type) {
		case *cst.FunctionDecl, *cst.MethodDecl, *cst.FunctionLit:
			index = -1
		case *cst.ForStmt:
			insideLoop = true
			index = -1
		}
	}
	if insideLoop {
		a.report("no-defer-in-loop", statement, "defer inside a loop runs at function exit, not iteration exit")
	}
	if !a.active("deferred-recover-call") && !a.active("discarded-deferred-result") {
		return
	}
	call, ok := statement.Expression.(*cst.PrimaryExpr)
	if !ok {
		return
	}
	if a.active("deferred-recover-call") && callName(call) == "recover" {
		a.report("deferred-recover-call", statement, "defer recover() evaluates recover immediately")
	}
	if !a.active("discarded-deferred-result") {
		return
	}
	cst.Walk(
		call.PrimaryExpr,
		func(node cst.Node) bool {
			literal,
				ok := node.(*cst.FunctionLit)
			if !ok || literal.Signature == nil || literal.Signature.Result == nil {
				return true
			}
			if declCount(resultDecls(literal.Signature.Result)) > 0 {
				a.report("discarded-deferred-result", statement, "return values from a deferred function are ignored")
			}
			return false
		},
	)
}

func (a *Pass) checkElseAfterReturn(statement *cst.IfElseStmt) {
	if !statement.ELSE.IsValid() || statement.Block == nil || !statementListEndsInReturn(statement.Block.StatementList) {
		return
	}
	a.report("no-else-after-return", statement.ELSE, "remove else and unindent its body after the return")
}

func statementListEndsInReturn(list *cst.StatementList) bool {
	var last cst.Node
	for ; list != nil; list = list.List {
		if list.Statement != nil {
			last = list.Statement
		}
	}
	_, ok := last.(*cst.ReturnStmt)
	return ok
}

func (a *Pass) checkPackageVar(declaration *cst.VarDecl) {
	for _, ancestor := range a.ancestors {
		switch ancestor.(type) {
		case *cst.FunctionDecl, *cst.MethodDecl, *cst.FunctionLit:
			return
		}
	}
	cst.Walk(
		declaration.VarSpec,
		func(node cst.Node) bool {
			switch spec := node.(type) {
			case *cst.VarSpec:
				if spec.IDENT.Src() != "_" {
					a.report("no-package-var", spec.IDENT, "package variables introduce mutable global state")
				}
				return false
			case *cst.VarSpec2:
				for names := spec.IdentifierList; names != nil; names = names.List {
					if names.IDENT.Src() != "_" {
						a.report("no-package-var", names.IDENT, "package variables introduce mutable global state")
					}
				}
				return false
			}
			return true
		},
	)
}
