package rules

import (
	"strings"

	"github.com/gempir/strider/internal/cst"
)

type concreteIf struct {
	node cst.Node
	init cst.Node
	condition cst.Node
	body *cst.Block
	elseToken cst.Token
	elseClause cst.Node
}

func (a *cstAnalyzer) checkConcreteIf(statement *cst.IfStmt) {
	if statement == nil {
		return
	}
	a.checkConcreteConditional(concreteIf{node: statement, init: statement.SimpleStmt, condition: statement.Expression, body: statement.Block})
}

func (a *cstAnalyzer) checkConcreteIfElse(statement *cst.IfElseStmt) {
	if statement == nil {
		return
	}
	a.checkConcreteConditional(
		concreteIf{node: statement, init: statement.SimpleStmt, condition: statement.Expression, body: statement.Block, elseToken: statement.ELSE, elseClause: statement.ElseClause},
	)
}

func (a *cstAnalyzer) checkConcreteConditional(statement concreteIf) {
	a.checkConcreteMapLookup(statement)
	if statement.init != nil {
		start, end := cst.Range(statement.init)
		if a.tree.Position(end).Line > a.tree.Position(start).Line {
			a.report("multiline-if-init", statement.init, "multiline if initializer should be moved above the if statement")
		}
	}
	if statement.elseClause != nil && concreteBlockTerminates(statement.body) {
		a.report("superfluous-else", statement.elseToken, "remove else after a branch that terminates control flow")
		if concreteBlockReturns(statement.body) {
			a.report("indent-error-flow", statement.elseToken, "drop else and outdent the block after the return")
		}
	}
	if elseBlock, ok := statement.elseClause.(*cst.Block); ok {
		if concreteBlockTerminates(elseBlock) && !concreteBlockTerminates(statement.body) {
			a.report("early-return", statement.node, "invert the condition and return early to reduce nesting")
		}
		if cst.Spelling(statement.body) == cst.Spelling(elseBlock) {
			a.report("identical-branches", elseBlock, "if and else branches are identical")
		}
		if left, leftOK := concreteBooleanBlock(statement.body); leftOK {
			if right, rightOK := concreteBooleanBlock(elseBlock); rightOK && left != right {
				a.report("unnecessary-if", statement.node, "return the condition directly instead of branching on it")
			}
		}
	}
	a.checkConcreteIfChain(statement)
}

func (a *cstAnalyzer) checkConcreteMapLookup(statement concreteIf) {
	declaration, ok := statement.init.(*cst.ShortVarDecl)
	if !ok || declaration.IdentifierList == nil || declaration.IdentifierList.Len() != 2 || declaration.ExpressionList == nil || declaration.ExpressionList.Len() != 1 || statement.body == nil {
		return
	}
	value := declaration.ExpressionList.Expression
	lookup := cst.Spelling(value)
	if !strings.Contains(lookup, "[") {
		return
	}
	names := concreteIdentifierTokens(declaration.IdentifierList)
	if len(names) != 2 || !concreteContainsIdentifier(statement.condition, names[1].Src()) {
		return
	}
	cst.Walk(
		statement.body,
		func(node cst.Node) bool {
			if cst.Spelling(node) == lookup {
				a.report("inefficient-map-lookup", node, "reuse the map value obtained by the comma-ok lookup")
				return false
			}
			return true
		},
	)
}

func (a *cstAnalyzer) checkConcreteIfChain(first concreteIf) {
	if len(a.ancestors) != 0 {
		if parent, ok := a.ancestors[len(a.ancestors) - 1].(*cst.IfElseStmt); ok && parent.ElseClause == first.node {
			return
		}
	}
	if first.init != nil || concreteHasSideEffects(first.condition) {
		return
	}
	conditions := map[string]bool{}
	branches := map[string]bool{}
	current := first
	for {
		condition := cst.Spelling(current.condition)
		if conditions[condition] {
			a.report("identical-ifelseif-conditions", current.condition, "if-else-if chain repeats a condition")
		} else {
			conditions[condition] = true
		}
		branch := cst.Spelling(current.body)
		if branches[branch] {
			a.report("identical-ifelseif-branches", current.body, "if-else-if chain repeats a branch body")
		} else {
			branches[branch] = true
		}
		next, ok := concreteIfFromNode(current.elseClause)
		if !ok || next.init != nil || concreteHasSideEffects(next.condition) {
			return
		}
		current = next
	}
}

func concreteIfFromNode(node cst.Node) (concreteIf, bool) {
	switch current := node.(type) {
	case *cst.IfStmt:
		return concreteIf{node:
		current, init:
		current.SimpleStmt, condition:
		current.Expression, body:
		current.Block}, true
	case *cst.IfElseStmt:
		return concreteIf{node:
		current, init:
		current.SimpleStmt, condition:
		current.Expression, body:
		current.Block, elseToken:
		current.ELSE, elseClause:
		current.ElseClause}, true
	default:
		return concreteIf{}, false
	}
}

func concreteHasSideEffects(node cst.Node) bool {
	found := false
	cst.Walk(
		node,
		func(child cst.Node) bool {
			if _,
			ok := child.(*cst.PrimaryExpr); ok && stringsHasArguments(child) {
				found = true
				return false
			}
			if unary,
			ok := child.(*cst.UnaryExpr); ok && unary.Op.Src() == "<-" {
				found = true
				return false
			}
			return !found
		},
	)
	return found
}

func stringsHasArguments(node cst.Node) bool {
	primary, ok := node.(*cst.PrimaryExpr)
	return ok && len(cst.Kind(primary.Postfix)) >= len("Arguments") && cst.Kind(primary.Postfix)[:len("Arguments")] == "Arguments"
}

func concreteBooleanBlock(block *cst.Block) (bool, bool) {
	statements := concreteBlockStatements(block)
	if len(statements) != 1 {
		return false, false
	}
	returned, ok := statements[0].(*cst.ReturnStmt)
	if !ok || returned.ExpressionList == nil || returned.ExpressionList.Len() != 1 {
		return false, false
	}
	value := cst.Spelling(returned.ExpressionList.Expression)
	return value == "true", value == "true" || value == "false"
}

func (a *cstAnalyzer) checkConcreteBlock(block *cst.Block) {
	statements := concreteBlockStatements(block)
	if len(statements) == 0 && a.concreteParentIsConditional() {
		a.report("empty-block", block, "empty block should be removed or documented")
	}
	for index, statement := range statements {
		if index + 1 < len(statements) {
			a.checkConcreteIfReturn(statement, statements[index + 1])
			a.checkConcreteWaitGroupAdd(statement, statements[index + 1])
		}
		if index > 0 && concreteStatementTerminates(statements[index - 1]) {
			a.report("unreachable-code", statement, "statement is unreachable after unconditional control flow")
		}
	}
	if len(statements) == 0 {
		return
	}
	openLine := block.LBRACE.Position().Line
	firstStart, _ := cst.Range(statements[0])
	_, lastEnd := cst.Range(statements[len(statements) - 1])
	closeLine := block.RBRACE.Position().Line
	if a.tree.Position(firstStart).Line > openLine + 1 {
		a.report("empty-lines", statements[0], "block begins with an unnecessary empty line")
	}
	if closeLine > a.tree.Position(lastEnd).Line + 1 {
		start, end := cst.Range(block.RBRACE)
		a.reportRange("empty-lines", start, end, "block ends with an unnecessary empty line")
	}
}

func (a *cstAnalyzer) concreteParentIsConditional() bool {
	if len(a.ancestors) == 0 {
		return false
	}
	switch a.ancestors[len(a.ancestors) - 1].(type) {
	case *cst.IfStmt, *cst.IfElseStmt:
		return true
	default:
		return false
	}
}

func concreteBlockStatements(block *cst.Block) []cst.Node {
	result := []cst.Node{}
	if block == nil {
		return result
	}
	for list := block.StatementList; list != nil; list = list.List {
		if list.Statement != nil {
			result = append(result, list.Statement)
		}
	}
	return result
}

func concreteBlockTerminates(block *cst.Block) bool {
	statements := concreteBlockStatements(block)
	return len(statements) != 0 && concreteStatementTerminates(statements[len(statements) - 1])
}

func concreteBlockReturns(block *cst.Block) bool {
	statements := concreteBlockStatements(block)
	if len(statements) == 0 {
		return false
	}
	_, ok := statements[len(statements) - 1].(*cst.ReturnStmt)
	return ok
}

func concreteStatementTerminates(statement cst.Node) bool {
	switch cst.Kind(statement) {
	case "ReturnStmt", "BreakStmt", "ContinueStmt", "GotoStmt", "FallthroughStmt":
		return true
	}
	if call, ok := statement.(*cst.PrimaryExpr); ok {
		name := concreteCallName(call)
		return name == "panic" || isDeepExit(name)
	}
	return false
}

func (a *cstAnalyzer) checkConcreteControlNesting(node cst.Node) {
	depth := 0
	for _, ancestor := range a.ancestors {
		switch cst.Kind(ancestor) {
		case "IfStmt", "IfElseStmt", "ForStmt", "ExprSwitchStmt", "TypeSwitchStmt", "SelectStmt":
			depth++
		}
	}
	if depth >= 5 {
		a.report("max-control-nesting", node, "control-flow nesting exceeds 5 levels")
	}
}

func (a *cstAnalyzer) checkConcreteTypeAssertion(assertion *cst.TypeAssertion) {
	if assertion.TypeNode == nil {
		return
	}
	search:
	for index := len(a.ancestors) - 1; index >= 0; index-- {
		if cst.Kind(a.ancestors[index]) == "TypeSwitchGuard" {
			return
		}
		switch parent := a.ancestors[index].(type) {
		case *cst.Assignment:
			if parent.ExpressionList != nil && parent.ExpressionList.Len() == 2 {
				return
			}
			break search
		case *cst.ShortVarDecl:
			if parent.IdentifierList != nil && parent.IdentifierList.Len() == 2 {
				return
			}
			break search
		}
	}
	a.report("unchecked-type-assertion", assertion, "use the checked two-result form of the type assertion")
}
