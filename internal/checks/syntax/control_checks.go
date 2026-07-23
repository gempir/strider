//strider:ignore-file cognitive-complexity,confusing-results,cyclomatic-complexity
package syntax

import (
	"strings"

	"github.com/gempir/strider/internal/cst"
)

type conditional struct {
	node       cst.Node
	init       cst.Node
	condition  cst.Node
	body       *cst.Block
	elseToken  cst.Token
	elseClause cst.Node
}

func conditionalFromIf(statement *cst.IfStmt) (conditional, bool) {
	if statement == nil {
		return conditional{}, false
	}
	return conditional{
		node:      statement,
		init:      statement.SimpleStmt,
		condition: statement.Expression,
		body:      statement.Block,
	}, true
}

func conditionalFromIfElse(statement *cst.IfElseStmt) (conditional, bool) {
	if statement == nil {
		return conditional{}, false
	}
	return conditional{
		node:       statement,
		init:       statement.SimpleStmt,
		condition:  statement.Expression,
		body:       statement.Block,
		elseToken:  statement.ELSE,
		elseClause: statement.ElseClause,
	}, true
}

func (a *Pass) checkEarlyReturn(statement conditional) {
	if elseBlock, ok := statement.elseClause.(*cst.Block); ok {
		if blockTerminates(elseBlock) && !blockTerminates(statement.body) {
			a.Report(statement.node, "invert the condition and return early to reduce nesting")
		}
	}
}

func (a *Pass) checkIdenticalBranches(statement conditional) {
	if elseBlock, ok := statement.elseClause.(*cst.Block); ok && cst.Spelling(statement.body) == cst.Spelling(elseBlock) {
		a.Report(elseBlock, "if and else branches are identical")
	}
}

func (a *Pass) checkUnnecessaryIf(statement conditional) {
	if elseBlock, ok := statement.elseClause.(*cst.Block); ok {
		if left, leftOK := booleanBlock(statement.body); leftOK {
			if right, rightOK := booleanBlock(elseBlock); rightOK && left != right {
				a.Report(statement.node, "return the condition directly instead of branching on it")
			}
		}
	}
}

func (a *Pass) checkInefficientMapLookup(statement conditional) {
	declaration, ok := statement.init.(*cst.ShortVarDecl)
	if !ok || declaration.IdentifierList == nil || declaration.IdentifierList.Len() != 2 || declaration.ExpressionList == nil || declaration.ExpressionList.Len() != 1 || statement.body == nil {
		return
	}
	value := declaration.ExpressionList.Expression
	lookup := cst.Spelling(value)
	if !strings.Contains(lookup, "[") {
		return
	}
	names := identifierTokens(declaration.IdentifierList)
	if len(names) != 2 || !containsIdentifier(statement.condition, names[1].Src()) {
		return
	}
	cst.Walk(
		statement.body,
		func(node cst.Node) bool {
			if cst.Spelling(node) == lookup {
				a.Report(node, "reuse the map value obtained by the comma-ok lookup")
				return false
			}
			return true
		},
	)
}

func (a *Pass) ifChain(first conditional) []conditional {
	if len(a.ancestors) != 0 {
		if parent, ok := a.ancestors[len(a.ancestors)-1].(*cst.IfElseStmt); ok && parent.ElseClause == first.node {
			return nil
		}
	}
	if first.init != nil || hasSideEffects(first.condition) {
		return nil
	}
	result := []conditional{}
	current := first
	for {
		result = append(result, current)
		next, ok := ifFromNode(current.elseClause)
		if !ok || next.init != nil || hasSideEffects(next.condition) {
			return result
		}
		current = next
	}
}

func (a *Pass) checkIdenticalIfChainConditions(first conditional) {
	seen := map[string]bool{}
	for _, current := range a.ifChain(first) {
		condition := cst.Spelling(current.condition)
		if seen[condition] {
			a.Report(current.condition, "if-else-if chain repeats a condition")
		}
		seen[condition] = true
	}
}

func (a *Pass) checkIdenticalIfChainBranches(first conditional) {
	seen := map[string]bool{}
	for _, current := range a.ifChain(first) {
		branch := cst.Spelling(current.body)
		if seen[branch] {
			a.Report(current.body, "if-else-if chain repeats a branch body")
		}
		seen[branch] = true
	}
}

func ifFromNode(node cst.Node) (conditional, bool) {
	switch current := node.(type) {
	case *cst.IfStmt:
		return conditional{
			node:      current,
			init:      current.SimpleStmt,
			condition: current.Expression,
			body:      current.Block,
		}, true
	case *cst.IfElseStmt:
		return conditional{
			node:       current,
			init:       current.SimpleStmt,
			condition:  current.Expression,
			body:       current.Block,
			elseToken:  current.ELSE,
			elseClause: current.ElseClause,
		}, true
	default:
		return conditional{}, false
	}
}

func hasSideEffects(node cst.Node) bool {
	found := false
	cst.Walk(
		node,
		func(child cst.Node) bool {
			if primary, ok := child.(*cst.PrimaryExpr); ok && cst.IsArguments(primary.Postfix) {
				found = true
				return false
			}
			if unary, ok := child.(*cst.UnaryExpr); ok && unary.Op.Src() == "<-" {
				found = true
				return false
			}
			return !found
		},
	)
	return found
}

func booleanBlock(block *cst.Block) (bool, bool) {
	statements := blockStatements(block)
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

func (a *Pass) checkEmptyConditionalBlock(block *cst.Block) {
	statements := blockStatements(block)
	if len(statements) == 0 && a.parentIsConditional() {
		a.Report(block, "empty block should be removed or documented")
	}
}

func (a *Pass) checkRedundantErrorReturn(block *cst.Block) {
	statements := blockStatements(block)
	for index, statement := range statements {
		if index+1 < len(statements) {
			a.checkIfReturn(statement, statements[index+1])
		}
	}
}

func (a *Pass) checkUnreachableCode(block *cst.Block) {
	statements := blockStatements(block)
	for index, statement := range statements {
		if index > 0 && statementTerminates(statements[index-1]) {
			a.Report(statement, "statement is unreachable after unconditional control flow")
		}
	}
}

func (a *Pass) parentIsConditional() bool {
	if len(a.ancestors) == 0 {
		return false
	}
	switch a.ancestors[len(a.ancestors)-1].(type) {
	case *cst.IfStmt, *cst.IfElseStmt:
		return true
	default:
		return false
	}
}

func blockStatements(block *cst.Block) []cst.Node {
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

func blockTerminates(block *cst.Block) bool {
	statements := blockStatements(block)
	return len(statements) != 0 && statementTerminates(statements[len(statements)-1])
}

func statementTerminates(statement cst.Node) bool {
	switch cst.Kind(statement) {
	case "ReturnStmt", "BreakStmt", "ContinueStmt", "GotoStmt", "FallthroughStmt":
		return true
	}
	if call, ok := statement.(*cst.PrimaryExpr); ok {
		name := callName(call)
		return name == "panic" || isDeepExit(name)
	}
	return false
}

func (a *Pass) checkControlNesting(node cst.Node) {
	depth := 0
	for _, ancestor := range a.ancestors {
		switch cst.Kind(ancestor) {
		case "IfStmt", "IfElseStmt", "ForStmt", "ExprSwitchStmt", "TypeSwitchStmt", "SelectStmt":
			depth++
		}
	}
	if depth >= 5 {
		a.Report(node, "control-flow nesting exceeds 5 levels")
	}
}

func (a *Pass) checkTypeAssertion(assertion *cst.TypeAssertion) {
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
			if parent.ExpressionList != nil && parent.ExpressionList.Len() == 2 && a.assertionIsSoleExpression(assertion, parent.ExpressionList2) {
				return
			}
			break search
		case *cst.ShortVarDecl:
			if parent.IdentifierList != nil && parent.IdentifierList.Len() == 2 && a.assertionIsSoleExpression(assertion, parent.ExpressionList) {
				return
			}
			break search
		}
	}
	a.Report(assertion, "use the checked two-result form of the type assertion")
}

func (a *Pass) assertionIsSoleExpression(assertion *cst.TypeAssertion, expressions *cst.ExpressionList) bool {
	if expressions == nil || expressions.Len() != 1 {
		return false
	}
	for index := len(a.ancestors) - 1; index >= 0; index-- {
		primary, ok := a.ancestors[index].(*cst.PrimaryExpr)
		if ok && primary.Postfix == assertion {
			return unparen(expressions.Expression) == primary
		}
	}
	return false
}
