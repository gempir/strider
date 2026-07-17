package rules

import (
	"sort"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

type concreteCase struct {
	node cst.Node
	conditions []cst.Node
	body *cst.StatementList
	isDefault bool
}

func (a *cstAnalyzer) checkConcreteSwitch(node cst.Node) {
	cases := concreteSwitchCases(node)
	if len(cases) == 1 && len(cases[0].conditions) <= 1 {
		a.report(
			"unnecessary-stmt",
			node,
			"switch with one case can be replaced by an if statement",
		)
	}
	conditions := map[string]bool{}
	branches := map[string]bool{}
	defaultIndex := -1
	for index, clause := range cases {
		if clause.isDefault {
			defaultIndex = index
		}
		for _, condition := range clause.conditions {
			text := cst.Spelling(condition)
			if conditions[text] {
				a.report(
					"identical-switch-conditions",
					condition,
					"switch repeats a case condition",
				)
			} else {
				conditions[text] = true
			}
		}
		body := concreteStatementListSpelling(clause.body)
		if body != "" {
			if branches[body] {
				a.report("identical-switch-branches", clause.node, "switch repeats a case body")
			} else {
				branches[body] = true
			}
		}
		if index == len(cases) - 1 {
			statements := concreteStatementsFromList(clause.body)
			if len(statements) != 0 && cst.Kind(statements[len(statements) - 1]) == "FallthroughStmt" {
				a.report(
					"useless-fallthrough",
					statements[len(statements) - 1],
					"fallthrough in the final switch case has no effect",
				)
			}
		}
	}
	if defaultIndex >= 0 && defaultIndex != len(cases) - 1 {
		a.report(
			"enforce-switch-style",
			cases[defaultIndex].node,
			"default clause should be the last switch clause",
		)
	}
}

func concreteSwitchCases(node cst.Node) []concreteCase {
	result := []concreteCase{}
	cst.Walk(
		node,
		func(child cst.Node) bool {
			switch clause := child.(type) {
			case *cst.ExprCaseClause:
				item := concreteCase{node:
				clause, body:
				clause.StatementList}
				if clause.ExprSwitchCase != nil {
					item.isDefault = strings.HasPrefix(
						cst.Spelling(clause.ExprSwitchCase),
						"default",
					)
					switch header := clause.ExprSwitchCase.(type) {
					case *cst.ExprSwitchCase:
						if header.Expression != nil {
							item.conditions = append(item.conditions, header.Expression)
						}
					case *cst.ExprSwitchCase2:
						for list := header.ExpressionList; list != nil; list = list.List {
							if list.Expression != nil {
								item.conditions = append(item.conditions, list.Expression)
							}
						}
					}
				}
				result = append(result, item)
				return false
			case *cst.TypeCaseClause:
				item := concreteCase{node:
				clause, body:
				clause.StatementList}
				if clause.TypeSwitchCase != nil {
					item.isDefault = clause.TypeSwitchCase.DEFAULT.IsValid()
					if clause.TypeSwitchCase.TypeList != nil {
						item.conditions = append(item.conditions, clause.TypeSwitchCase.TypeList)
					}
				}
				result = append(result, item)
				return false
			}
			return true
		},
	)
	sort.SliceStable(
		result,
		func(i, j int) bool {
			left,
			_ := cst.Range(result[i].node)
			right,
			_ := cst.Range(result[j].node)
			return left < right
		},
	)
	return result
}

func concreteStatementListSpelling(list *cst.StatementList) string {
	parts := []string{}
	for _, statement := range concreteStatementsFromList(list) {
		parts = append(parts, cst.Spelling(statement))
	}
	return strings.Join(parts, "\n")
}

func concreteStatementsFromList(list *cst.StatementList) []cst.Node {
	result := []cst.Node{}
	for; list != nil; list = list.List {
		if list.Statement != nil {
			result = append(result, list.Statement)
		}
	}
	return result
}
