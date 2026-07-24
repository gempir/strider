//strider:ignore-file cognitive-complexity,confusing-naming,cyclomatic-complexity,identical-switch-branches,modifies-parameter,single-case-switch
package syntax

import (
	"go/token"

	"github.com/gempir/strider/internal/checks/catalog"
	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/telemetry"
)

type Pass struct {
	filename  string
	tree      *cst.Tree
	content   []byte
	reporter  func(Finding)
	ancestors []cst.Node
	current   cst.Node
	check     Check
	options   map[string]catalog.ResolvedOptions
	states    checkStates
	functions map[cst.Node]*functionFacts
	calls     map[*cst.PrimaryExpr]callFacts
	stats     executionStats
}

type executionStats struct {
	functionFacts int
}

// AnalyzeCST runs selected native CST checks over one lossless source tree.
func AnalyzeCST(input CSTInput) {
	analyzeCST(input)
}

func analyzeCST(input CSTInput) executionStats {
	if len(input.Checks) == 0 || input.Tree == nil {
		return executionStats{}
	}
	finish := telemetry.Start("syntax.traversal")
	defer finish()
	analyzer := &Pass{
		filename:  input.Filename,
		tree:      input.Tree,
		content:   input.Tree.Bytes(),
		reporter:  input.Report,
		options:   input.Options,
		functions: make(map[cst.Node]*functionFacts),
		calls:     make(map[*cst.PrimaryExpr]callFacts),
	}
	dispatch := input.Dispatch
	if dispatch == nil {
		dispatch = buildDispatch(input.Checks)
	}
	for _, check := range input.Checks {
		analyzer.run(check, check.Start)
	}
	cst.WalkProductionsWithAncestors(
		input.Tree.Root(),
		func(node cst.Node, ancestors []cst.Node) bool {
			analyzer.ancestors = ancestors
			analyzer.current = node
			analyzer.dispatch(dispatch[NodeKind(cst.Kind(node))], node)
			return true
		},
	)
	for _, check := range input.Checks {
		analyzer.run(check, check.Finish)
	}
	return analyzer.stats
}

func buildDispatch(checks []Check) map[NodeKind][]Check {
	dispatch := make(map[NodeKind][]Check)
	for _, check := range checks {
		for _, interest := range check.Interests() {
			dispatch[interest] = append(dispatch[interest], check)
		}
	}
	return dispatch
}

func (a *Pass) dispatch(checks []Check, node cst.Node) {
	for _, check := range checks {
		a.run(check, func(pass *Pass) {
			check.Inspect(pass, node)
		})
	}
}

func (a *Pass) run(check Check, execute func(*Pass)) {
	a.check = check
	execute(a)
	a.check = nil
}

func (a *Pass) currentCode() string {
	if a.check == nil {
		return ""
	}
	return a.check.Meta().Code
}

// Report emits a finding owned by the currently executing descriptor.
func (a *Pass) Report(node cst.Node, message string) {
	if a.check == nil || node == nil || a.reporter == nil {
		return
	}
	a.reporter(Finding{
		Node:    node,
		Code:    a.check.Meta().Code,
		Message: message,
	})
}

// ReportFix emits a finding and fix owned by the current descriptor.
func (a *Pass) ReportFix(node cst.Node, message string, fix diagnostic.Fix) {
	if a.check == nil || node == nil || a.reporter == nil {
		return
	}
	a.reporter(Finding{
		Node:    node,
		Code:    a.check.Meta().Code,
		Message: message,
		Fixes: []diagnostic.Fix{
			fix,
		},
	})
}

// ReportRange emits an offset finding owned by the current descriptor.
func (a *Pass) ReportRange(start, end int, message string) {
	if a.check == nil || a.reporter == nil {
		return
	}
	a.reporter(Finding{
		Start:    start,
		End:      end,
		HasRange: true,
		Code:     a.check.Meta().Code,
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

func (a *Pass) intOption(name string) int {
	if a.check == nil {
		return 0
	}
	value, _ := a.options[a.check.Meta().Code].Int(name)
	return value
}

func (a *Pass) stringsOption(name string) []string {
	if a.check == nil {
		return nil
	}
	value, _ := a.options[a.check.Meta().Code].Strings(name)
	return value
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
	a.Report(statement, "return values must be explicit")
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

func (a *Pass) checkDeferInLoop(statement *cst.DeferStmt) {
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
		a.Report(statement, "defer inside a loop runs at function exit, not iteration exit")
	}
}

func (a *Pass) checkDeferredRecoverCall(statement *cst.DeferStmt) {
	call, ok := statement.Expression.(*cst.PrimaryExpr)
	if !ok {
		return
	}
	if callName(call) == "recover" {
		a.Report(statement, "defer recover() evaluates recover immediately")
	}
}

func (a *Pass) checkDiscardedDeferredResult(statement *cst.DeferStmt) {
	call, ok := statement.Expression.(*cst.PrimaryExpr)
	if !ok {
		return
	}
	cst.Walk(
		call.PrimaryExpr,
		func(node cst.Node) bool {
			literal, ok := node.(*cst.FunctionLit)
			if !ok || literal.Signature == nil || literal.Signature.Result == nil {
				return true
			}
			if declCount(resultDecls(literal.Signature.Result)) > 0 {
				a.Report(statement, "return values from a deferred function are ignored")
			}
			return false
		},
	)
}

func (a *Pass) checkElseAfterReturn(statement *cst.IfElseStmt) {
	if !statement.ELSE.IsValid() || statement.Block == nil || !statementListEndsInReturn(statement.Block.StatementList) {
		return
	}
	a.Report(statement.ELSE, "remove else and unindent its body after the return")
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
					a.Report(spec.IDENT, "package variables introduce mutable global state")
				}
				return false
			case *cst.VarSpec2:
				for names := spec.IdentifierList; names != nil; names = names.List {
					if names.IDENT.Src() != "_" {
						a.Report(names.IDENT, "package variables introduce mutable global state")
					}
				}
				return false
			}
			return true
		},
	)
}
