//strider:ignore-file cognitive-complexity,cyclomatic-complexity
package syntax

import (
	"fmt"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

func (a *Pass) checkSpinningSelectDefault(statement *cst.ForStmt) {
	if statement == nil || statement.RangeClause != nil {
		return
	}
	if statement.ForClause != nil || statement.Condition != nil || statement.Block == nil {
		return
	}
	statements := blockStatements(statement.Block)
	if len(statements) != 1 {
		return
	}
	selection, ok := statements[0].(*cst.SelectStmt)
	if !ok {
		return
	}
	for list := selection.CommClauseList; list != nil; list = list.List {
		clause := list.CommClause
		if clause != nil && clause.CommCase != nil && clause.CommCase.DEFAULT.IsValid() && len(statementsFromList(clause.StatementList)) == 0 {
			a.Report(clause, "empty default prevents the select loop from blocking and causes it to spin")
			return
		}
	}
}

func (a *Pass) checkSimplifyRange(statement *cst.ForStmt) {
	if statement == nil || statement.RangeClause == nil {
		return
	}
	clause := statement.RangeClause
	variables := rangeVariables(clause)
	if rangeIndexDiscarded(clause) && strings.HasPrefix(cst.Spelling(clause.Expression), "[]rune(") {
		a.Report(clause.Expression, "range directly over the string to avoid allocating a rune slice")
	}
	if len(variables) > 1 && variables[1].Src() == "_" {
		a.Report(variables[1], "omit the blank range value")
	}
}

func (a *Pass) checkRangeValueAddress(statement *cst.ForStmt) {
	if statement == nil || statement.RangeClause == nil {
		return
	}
	variables := rangeVariables(statement.RangeClause)
	for _, variable := range variables {
		name := variable.Src()
		if name == "" || name == "_" {
			continue
		}
		cst.Walk(
			statement.Block,
			func(node cst.Node) bool {
				if current, ok := node.(*cst.UnaryExpr); ok {
					if current.Op.Src() == "&" && containsIdentifier(current.UnaryExpr, name) {
						a.Report(current, fmt.Sprintf("taking the address of range value %s can be misleading", name))
					}
				}
				return true
			},
		)
	}
}

func rangeVariables(clause *cst.RangeClause) []cst.Token {
	if clause == nil {
		return nil
	}
	if clause.IdentifierList != nil {
		return identifierTokens(clause.IdentifierList)
	}
	result := []cst.Token{}
	for list := clause.ExpressionList; list != nil; list = list.List {
		if token := singleIdentifier(list.Expression); token.IsValid() {
			result = append(result, token)
		}
	}
	return result
}

func rangeIndexDiscarded(clause *cst.RangeClause) bool {
	variables := rangeVariables(clause)
	return len(variables) == 0 || variables[0].Src() == "_"
}

func singleIdentifier(node cst.Node) cst.Token {
	if node == nil {
		return cst.Token{}
	}
	tokens := cst.NodeTokens(node)
	if len(tokens) == 1 && tokens[0].Src() != "" {
		return tokens[0]
	}
	return cst.Token{}
}

func containsIdentifier(node cst.Node, name string) bool {
	for _, current := range cst.NodeTokens(node) {
		if current.Src() == name {
			return true
		}
	}
	return false
}
