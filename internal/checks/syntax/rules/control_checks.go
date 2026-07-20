package rules

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

func (a *Pass) checkIf(statement *cst.IfStmt) {
	if statement == nil {
		return
	}
	a.checkConditional(conditional{
		node:      statement,
		init:      statement.SimpleStmt,
		condition: statement.Expression,
		body:      statement.Block,
	})
}

func (a *Pass) checkIfElse(statement *cst.IfElseStmt) {
	if statement == nil {
		return
	}
	a.checkConditional(
		conditional{
			node:       statement,
			init:       statement.SimpleStmt,
			condition:  statement.Expression,
			body:       statement.Block,
			elseToken:  statement.ELSE,
			elseClause: statement.ElseClause,
		},
	)
}

func (a *Pass) checkConditional(statement conditional) {
	a.checkMapLookup(statement)
	if elseBlock, ok := statement.elseClause.(*cst.Block); ok {
		if blockTerminates(elseBlock) && !blockTerminates(statement.body) {
			a.report("early-return", statement.node, "invert the condition and return early to reduce nesting")
		}
		if cst.Spelling(statement.body) == cst.Spelling(elseBlock) {
			a.report("identical-branches", elseBlock, "if and else branches are identical")
		}
		if left, leftOK := booleanBlock(statement.body); leftOK {
			if right, rightOK := booleanBlock(elseBlock); rightOK && left != right {
				a.report("unnecessary-if", statement.node, "return the condition directly instead of branching on it")
			}
		}
	}
	a.checkIfChain(statement)
}

func (a *Pass) checkMapLookup(statement conditional) {
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
				a.report("inefficient-map-lookup", node, "reuse the map value obtained by the comma-ok lookup")
				return false
			}
			return true
		},
	)
}

func (a *Pass) checkIfChain(first conditional) {
	if len(a.ancestors) != 0 {
		if parent, ok := a.ancestors[len(a.ancestors)-1].(*cst.IfElseStmt); ok && parent.ElseClause == first.node {
			return
		}
	}
	if first.init != nil || hasSideEffects(first.condition) {
		return
	}
	conditions := map[string]bool{}
	branches := map[string]bool{}
	current := first
	for {
		condition := cst.Spelling(current.condition)
		if conditions[condition] {
			a.report("identical-if-chain-conditions", current.condition, "if-else-if chain repeats a condition")
		} else {
			conditions[condition] = true
		}
		branch := cst.Spelling(current.body)
		if branches[branch] {
			a.report("identical-if-chain-branches", current.body, "if-else-if chain repeats a branch body")
		} else {
			branches[branch] = true
		}
		next, ok := ifFromNode(current.elseClause)
		if !ok || next.init != nil || hasSideEffects(next.condition) {
			return
		}
		current = next
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
			if _,
				ok := child.(*cst.PrimaryExpr); ok && hasArguments(child) {
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

func hasArguments(node cst.Node) bool {
	primary, ok := node.(*cst.PrimaryExpr)
	if !ok {
		return false
	}
	switch primary.Postfix.(type) {
	case *cst.Arguments, *cst.Arguments1, *cst.Arguments2, *cst.Arguments3:
		return true
	default:
		return false
	}
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

func (a *Pass) checkBlock(block *cst.Block) {
	statements := blockStatements(block)
	if len(statements) == 0 && a.parentIsConditional() {
		a.report("empty-conditional-block", block, "empty block should be removed or documented")
	}
	for index, statement := range statements {
		if index+1 < len(statements) {
			a.checkIfReturn(statement, statements[index+1])
		}
		if index > 0 && statementTerminates(statements[index-1]) {
			a.report("unreachable-code", statement, "statement is unreachable after unconditional control flow")
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
		a.report("max-control-nesting", node, "control-flow nesting exceeds 5 levels")
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
	a.report("unchecked-type-assertion", assertion, "use the checked two-result form of the type assertion")
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
