package rules

import (
	"go/ast"
	"go/token"
)

func (a *analyzer) checkIf(statement *ast.IfStmt) {
	a.checkNoElseAfterReturn(statement)
	if statement.Init != nil {
		start := a.fset.Position(statement.Init.Pos()).Line
		end := a.fset.Position(statement.Init.End()).Line
		if end > start {
			a.report(
				"multiline-if-init",
				statement.Init,
				"multiline if initializer should be moved above the if statement",
			)
		}
	}
	if statement.Else != nil && blockTerminates(statement.Body) {
		a.report(
			"superfluous-else",
			statement.Else,
			"remove else after a branch that terminates control flow",
		)
		if blockReturns(statement.Body) {
			a.report(
				"indent-error-flow",
				statement.Else,
				"drop else and outdent the block after the return",
			)
		}
	}
	if elseBlock, ok := statement.Else.(*ast.BlockStmt); ok && blockTerminates(elseBlock) &&
		!blockTerminates(statement.Body) {
		a.report(
			"early-return",
			statement,
			"invert the condition and return early to reduce nesting",
		)
	}
	if mapIndex, key := mapLookupInitializer(statement.Init); mapIndex != "" &&
		expressionContainsIdent(statement.Cond, key) {
		ast.Inspect(
			statement.Body,
			func(node ast.Node) bool {
				index, ok := node.(*ast.IndexExpr)
				if ok && nodeText(index) == mapIndex {
					a.report(
						"inefficient-map-lookup",
						index,
						"reuse the map value obtained by the comma-ok lookup",
					)
					return false
				}
				return true
			},
		)
	}
	if elseBlock, ok := statement.Else.(*ast.BlockStmt); ok &&
		nodesEquivalent(statement.Body, elseBlock) {
		a.report("identical-branches", elseBlock, "if and else branches are identical")
	}
	a.checkIfChain(statement)
	a.checkBooleanReturn(statement)
}

func mapLookupInitializer(statement ast.Stmt) (string, string) {
	assignment, ok := statement.(*ast.AssignStmt)
	if !ok || len(assignment.Lhs) != 2 || len(assignment.Rhs) != 1 {
		return "", ""
	}
	index, ok := assignment.Rhs[0].(*ast.IndexExpr)
	if !ok {
		return "", ""
	}
	condition, ok := assignment.Lhs[1].(*ast.Ident)
	if !ok {
		return "", ""
	}
	return nodeText(index), condition.Name
}

func (a *analyzer) checkIfChain(statement *ast.IfStmt) {
	if parent, ok := a.parent().(*ast.IfStmt); ok && parent.Else == statement {
		return
	}
	conditions := map[string]ast.Expr{}
	branches := map[string]*ast.BlockStmt{}
	for current := statement; current != nil; {
		if current.Init != nil || expressionHasPotentialSideEffects(current.Cond) {
			return
		}
		condition := nodeText(current.Cond)
		if first := conditions[condition]; first != nil {
			a.report(
				"identical-ifelseif-conditions",
				current.Cond,
				"if-else-if chain repeats a condition",
			)
		} else {
			conditions[condition] = current.Cond
		}
		branch := nodeText(current.Body)
		if first := branches[branch]; first != nil {
			a.report(
				"identical-ifelseif-branches",
				current.Body,
				"if-else-if chain repeats a branch body",
			)
		} else {
			branches[branch] = current.Body
		}
		next, ok := current.Else.(*ast.IfStmt)
		if !ok {
			break
		}
		current = next
	}
}

func expressionHasPotentialSideEffects(expression ast.Expr) bool {
	hasSideEffects := false
	ast.Inspect(expression, func(node ast.Node) bool {
		if hasSideEffects {
			return false
		}
		switch node := node.(type) {
		case *ast.CallExpr:
			hasSideEffects = true
			return false
		case *ast.UnaryExpr:
			if node.Op == token.ARROW {
				hasSideEffects = true
				return false
			}
		}
		return true
	})
	return hasSideEffects
}

func (a *analyzer) checkBooleanReturn(statement *ast.IfStmt) {
	if statement.Else == nil || len(statement.Body.List) != 1 {
		return
	}
	left, ok := booleanReturn(statement.Body.List[0])
	if !ok {
		return
	}
	var right bool
	switch otherwise := statement.Else.(type) {
	case *ast.BlockStmt:
		if len(otherwise.List) != 1 {
			return
		}
		right, ok = booleanReturn(otherwise.List[0])
	default:
		return
	}
	if ok && left != right {
		a.report(
			"unnecessary-if",
			statement,
			"return the condition directly instead of branching on it",
		)
	}
}

func booleanReturn(statement ast.Stmt) (bool, bool) {
	ret, ok := statement.(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false, false
	}
	id, ok := ret.Results[0].(*ast.Ident)
	if !ok || (id.Name != "true" && id.Name != "false") {
		return false, false
	}
	return id.Name == "true", true
}

func blockTerminates(block *ast.BlockStmt) bool {
	if block == nil || len(block.List) == 0 {
		return false
	}
	return statementTerminates(block.List[len(block.List)-1])
}

func blockReturns(block *ast.BlockStmt) bool {
	if block == nil || len(block.List) == 0 {
		return false
	}
	_, ok := block.List[len(block.List)-1].(*ast.ReturnStmt)
	return ok
}

func statementTerminates(statement ast.Stmt) bool {
	switch n := statement.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return n.Tok == token.BREAK || n.Tok == token.CONTINUE || n.Tok == token.GOTO ||
			n.Tok == token.FALLTHROUGH
	case *ast.ExprStmt:
		call, ok := n.X.(*ast.CallExpr)
		return ok && (callName(call) == "panic" || isDeepExit(callName(call)))
	}
	return false
}

func nodesEquivalent(left, right ast.Node) bool {
	return nodeText(left) == nodeText(right)
}
