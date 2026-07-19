package rules

import (
	"fmt"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

func (a *cstAnalyzer) checkConcreteFor(statement *cst.ForStmt) {
	if statement == nil {
		return
	}
	if statement.RangeClause != nil {
		a.checkConcreteRange(statement.RangeClause, statement.Block)
		return
	}
	if statement.ForClause != nil || statement.Condition != nil || statement.Block == nil {
		return
	}
	statements := concreteBlockStatements(statement.Block)
	if len(statements) != 1 {
		return
	}
	selection, ok := statements[0].(*cst.SelectStmt)
	if !ok {
		return
	}
	for list := selection.CommClauseList; list != nil; list = list.List {
		clause := list.CommClause
		if clause != nil && clause.CommCase != nil && clause.CommCase.DEFAULT.IsValid() && len(concreteStatementsFromList(clause.StatementList)) == 0 {
			a.report("spinning-select-default", clause, "empty default prevents the select loop from blocking and causes it to spin")
			return
		}
	}
}

func (a *cstAnalyzer) checkConcreteRange(clause *cst.RangeClause, body *cst.Block) {
	variables := concreteRangeVariables(clause)
	if concreteRangeIndexDiscarded(clause) && strings.HasPrefix(cst.Spelling(clause.Expression), "[]rune(") {
		a.report("simplify-range", clause.Expression, "range directly over the string to avoid allocating a rune slice")
	}
	if len(variables) > 1 && variables[1].Src() == "_" {
		a.report("simplify-range", variables[1], "omit the blank range value")
	}
	for _, variable := range variables {
		name := variable.Src()
		if name == "" || name == "_" {
			continue
		}
		cst.Walk(
			body,
			func(node cst.Node) bool {
				if current,
					ok := node.(*cst.UnaryExpr); ok {
					if current.Op.Src() == "&" && concreteContainsIdentifier(current.UnaryExpr, name) {
						a.report("range-value-address", current, fmt.Sprintf("taking the address of range value %s can be misleading", name))
					}
				}
				return true
			},
		)
	}
}

func concreteRangeVariables(clause *cst.RangeClause) []cst.Token {
	if clause == nil {
		return nil
	}
	if clause.IdentifierList != nil {
		return concreteIdentifierTokens(clause.IdentifierList)
	}
	result := []cst.Token{}
	for list := clause.ExpressionList; list != nil; list = list.List {
		if token := concreteSingleIdentifier(list.Expression); token.IsValid() {
			result = append(result, token)
		}
	}
	return result
}

func concreteRangeIndexDiscarded(clause *cst.RangeClause) bool {
	variables := concreteRangeVariables(clause)
	return len(variables) == 0 || variables[0].Src() == "_"
}

func concreteSingleIdentifier(node cst.Node) cst.Token {
	if node == nil {
		return cst.Token{}
	}
	tokens := cst.NodeTokens(node)
	if len(tokens) == 1 && tokens[0].Src() != "" {
		return tokens[0]
	}
	return cst.Token{}
}

func concreteContainsIdentifier(node cst.Node, name string) bool {
	for _, current := range cst.NodeTokens(node) {
		if current.Src() == name {
			return true
		}
	}
	return false
}
