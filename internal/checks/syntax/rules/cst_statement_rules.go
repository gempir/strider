package rules

import (
	"strings"

	"github.com/gempir/strider/internal/cst"
)

func (a *cstAnalyzer) checkConcreteIfReturn(current, next cst.Node) {
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
	body := concreteBlockStatements(statement.Block)
	returned, ok := concreteSingleReturn(body)
	if !ok || cst.Spelling(returned.ExpressionList) != name {
		return
	}
	final, ok := next.(*cst.ReturnStmt)
	if ok && final.ExpressionList != nil && final.ExpressionList.Len() == 1 && cst.Spelling(final.ExpressionList.Expression) == "nil" {
		a.report("if-return", statement, "return the error directly instead of checking it before returning")
	}
}

func concreteSingleReturn(statements []cst.Node) (*cst.ReturnStmt, bool) {
	if len(statements) != 1 {
		return nil, false
	}
	returned, ok := statements[0].(*cst.ReturnStmt)
	return returned, ok && returned.ExpressionList != nil && returned.ExpressionList.Len() == 1
}

func (a *cstAnalyzer) checkConcreteWaitGroupAdd(current, next cst.Node) {
	call, ok := current.(*cst.PrimaryExpr)
	if !ok || !strings.HasSuffix(concreteCallName(call), ".Add") {
		return
	}
	arguments := concreteCallArguments(call.Postfix)
	if len(arguments) != 1 || cst.Spelling(arguments[0]) != "1" || cst.Kind(next) != "GoStmt" {
		return
	}
	a.report("use-waitgroup-go", call, "replace WaitGroup.Add followed by go with WaitGroup.Go")
}

func (a *cstAnalyzer) checkConcreteBreak(statement *cst.BreakStmt) {
	if statement == nil || statement.Label != nil {
		return
	}
	for index := len(a.ancestors) - 1; index >= 0; index-- {
		var list *cst.StatementList
		switch clause := a.ancestors[index].(type) {
		case *cst.ExprCaseClause:
			list = clause.StatementList
		case *cst.TypeCaseClause:
			list = clause.StatementList
		default:
			continue
		}
		statements := concreteStatementsFromList(list)
		if len(statements) != 0 && statements[len(statements)-1] == statement {
			a.report("useless-break", statement, "break at the end of a switch case is redundant")
			a.report("unnecessary-stmt", statement, "omit unnecessary break at the end of a case clause")
		}
		return
	}
}

func (a *cstAnalyzer) checkConcreteTestMain(function *cst.FunctionDecl) {
	if function == nil || function.FunctionName == nil || function.FunctionName.IDENT.Src() != "TestMain" || !strings.HasSuffix(a.filename, "_test.go") || function.FunctionBody == nil {
		return
	}
	cst.Walk(
		function.FunctionBody,
		func(node cst.Node) bool {
			call,
				ok := node.(*cst.PrimaryExpr)
			if ok && concreteCallName(call) == "os.Exit" {
				a.report("redundant-test-main-exit", call, "TestMain can return instead of calling os.Exit")
			}
			return true
		},
	)
}
