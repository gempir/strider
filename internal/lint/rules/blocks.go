package rules

import (
	"go/ast"
	"go/token"
	"strings"
)

func (a *analyzer) checkBlock(block *ast.BlockStmt) {
	if len(block.List) == 0 && a.emptyConditionalBranch(block) {
		a.report("empty-block", block, "empty block should be removed or documented")
	}
	for index, statement := range block.List {
		if index+1 < len(block.List) {
			a.checkRedundantErrorIf(statement, block.List[index+1])
		}
		if index+1 < len(block.List) {
			if add, ok := waitGroupAdd(statement); ok {
				if _, ok := block.List[index+1].(*ast.GoStmt); ok {
					a.report(
						"use-waitgroup-go",
						add,
						"replace WaitGroup.Add followed by go with WaitGroup.Go",
					)
				}
			}
		}
		if index > 0 && statementTerminates(block.List[index-1]) {
			a.report(
				"unreachable-code",
				statement,
				"statement is unreachable after unconditional control flow",
			)
		}
		if index == len(block.List)-1 {
			if branch, ok := statement.(*ast.BranchStmt); ok && branch.Tok == token.BREAK {
				if _, ok := a.nearestControl().(*ast.SwitchStmt); ok {
					a.report(
						"useless-break",
						branch,
						"break at the end of a switch case is redundant",
					)
				}
			}
		}
	}
	a.checkBlockWhitespace(block)
}

func (a *analyzer) emptyConditionalBranch(block *ast.BlockStmt) bool {
	conditional, ok := a.parent().(*ast.IfStmt)
	return ok && (conditional.Body == block || conditional.Else == block)
}

func (a *analyzer) checkRedundantErrorIf(current, next ast.Stmt) {
	statement, ok := current.(*ast.IfStmt)
	if !ok || statement.Else != nil || len(statement.Body.List) != 1 {
		return
	}
	assignment, ok := statement.Init.(*ast.AssignStmt)
	if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return
	}
	id, ok := assignment.Lhs[0].(*ast.Ident)
	if !ok {
		return
	}
	condition, ok := statement.Cond.(*ast.BinaryExpr)
	if !ok || condition.Op != token.NEQ || nodeText(condition.X) != id.Name ||
		nodeText(condition.Y) != "nil" {
		return
	}
	returned, ok := statement.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(returned.Results) != 1 || nodeText(returned.Results[0]) != id.Name {
		return
	}
	final, ok := next.(*ast.ReturnStmt)
	if ok && len(final.Results) == 1 && nodeText(final.Results[0]) == "nil" {
		a.report(
			"if-return",
			statement,
			"return the error directly instead of checking it before returning",
		)
	}
}

func waitGroupAdd(statement ast.Stmt) (*ast.CallExpr, bool) {
	expr, ok := statement.(*ast.ExprStmt)
	if !ok {
		return nil, false
	}
	call, ok := expr.X.(*ast.CallExpr)
	if !ok || !strings.HasSuffix(callName(call), ".Add") || len(call.Args) != 1 ||
		!isOne(call.Args[0]) {
		return nil, false
	}
	return call, true
}

func (a *analyzer) checkBlockWhitespace(block *ast.BlockStmt) {
	if len(block.List) == 0 {
		return
	}
	openLine := a.fset.Position(block.Lbrace).Line
	firstLine := a.fset.Position(block.List[0].Pos()).Line
	closeLine := a.fset.Position(block.Rbrace).Line
	lastLine := a.fset.Position(block.List[len(block.List)-1].End()).Line
	if firstLine > openLine+1 {
		a.report("empty-lines", block.List[0], "block begins with an unnecessary empty line")
	}
	if closeLine > lastLine+1 {
		a.report(
			"empty-lines",
			positionedNode{block.Rbrace - 1, block.Rbrace},
			"block ends with an unnecessary empty line",
		)
	}
}

func (a *analyzer) nearestControl() ast.Node {
	for index := len(a.ancestors) - 1; index >= 0; index-- {
		switch n := a.ancestors[index].(type) {
		case *ast.SwitchStmt:
			return n
		case *ast.TypeSwitchStmt:
			return n
		case *ast.ForStmt:
			return n
		case *ast.RangeStmt:
			return n
		}
	}
	return nil
}
