//strider:ignore-file cyclomatic-complexity,identical-switch-branches
package syntax

import (
	"bytes"

	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/diagnostic"
)

func (a *Pass) checkIfReturn(current, next cst.Node) {
	statement, ok := current.(*cst.IfStmt)
	if !ok || statement.SimpleStmt == nil || statement.Block == nil {
		return
	}
	assignment, ok := statement.SimpleStmt.(*cst.ShortVarDecl)
	if !ok || assignment.IdentifierList == nil || assignment.IdentifierList.Len() != 1 || assignment.ExpressionList == nil || assignment.ExpressionList.Len() != 1 {
		return
	}
	name := assignment.IdentifierList.IDENT.Src()
	if cst.Spelling(statement.Expression) != name+"!=nil" {
		return
	}
	body := blockStatements(statement.Block)
	returned, ok := singleReturn(body)
	if !ok || cst.Spelling(returned.ExpressionList) != name {
		return
	}
	final, ok := next.(*cst.ReturnStmt)
	if ok && final.ExpressionList != nil && final.ExpressionList.Len() == 1 && cst.Spelling(final.ExpressionList.Expression) == "nil" {
		a.Report(statement, "return the error directly instead of checking it before returning")
	}
}

func singleReturn(statements []cst.Node) (*cst.ReturnStmt, bool) {
	if len(statements) != 1 {
		return nil, false
	}
	returned, ok := statements[0].(*cst.ReturnStmt)
	return returned, ok && returned.ExpressionList != nil && returned.ExpressionList.Len() == 1
}

func (a *Pass) checkBreak(statement *cst.BreakStmt) {
	if statement == nil || statement.Label != nil {
		return
	}
	for index := len(a.ancestors) - 1; index >= 0; index-- {
		var list *cst.StatementList
		switch clause := a.ancestors[index].(type) {
		case *cst.ExprCaseClauseList:
			list = clause.StatementList
		case *cst.TypeCaseClause:
			list = clause.StatementList
		default:
			continue
		}
		statements := statementsFromList(list)
		if len(statements) != 0 && statements[len(statements)-1] == statement {
			start, end := cst.Range(statement)
			edit := a.redundantBreakEdit(start, end)
			a.ReportFix(
				statement,
				"omit unnecessary break at the end of a case clause",
				diagnostic.Fix{
					Message:   "remove the redundant break",
					Safety:    diagnostic.Safe,
					Automatic: true,
					Edits: []diagnostic.TextEdit{
						edit,
					},
				},
			)
		}
		return
	}
}

func (a *Pass) redundantBreakEdit(start, end int) diagnostic.TextEdit {
	edit := diagnostic.TextEdit{
		Start:   start,
		End:     end,
		OldText: "break",
	}
	lineStart := bytes.LastIndexByte(a.content[:start], '\n') + 1
	newline := bytes.IndexByte(a.content[end:], '\n')
	if newline < 0 {
		return edit
	}
	lineEnd := end + newline + 1
	before := bytes.TrimSpace(a.content[lineStart:start])
	after := bytes.TrimSpace(a.content[end : lineEnd-1])
	if len(before) != 0 || len(after) != 0 && !bytes.Equal(after, []byte(";")) {
		return edit
	}
	return diagnostic.TextEdit{
		Start:   lineStart,
		End:     lineEnd,
		OldText: string(a.content[lineStart:lineEnd]),
	}
}
