package syntax

import (
	"sort"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

type switchCase struct {
	node       cst.Node
	conditions []cst.Node
	body       *cst.StatementList
	isDefault  bool
}

func (a *Pass) checkSingleCaseSwitch(node cst.Node) {
	cases := switchCases(node)
	if len(cases) == 1 && len(cases[0].conditions) <= 1 {
		a.Report(node, "switch with one case can be replaced by an if statement")
	}
}

func (a *Pass) checkIdenticalSwitchConditions(node cst.Node) {
	conditions := map[string]bool{}
	for _, clause := range switchCases(node) {
		for _, condition := range clause.conditions {
			text := cst.Spelling(condition)
			if conditions[text] {
				a.Report(condition, "switch repeats a case condition")
			} else {
				conditions[text] = true
			}
		}
	}
}

func (a *Pass) checkIdenticalSwitchBranches(node cst.Node) {
	branches := map[string]bool{}
	for _, clause := range switchCases(node) {
		body := statementListSpelling(clause.body)
		if body != "" {
			if branches[body] {
				a.Report(clause.node, "switch repeats a case body")
			} else {
				branches[body] = true
			}
		}
	}
}

func (a *Pass) checkSwitchDefaultLast(node cst.Node) {
	cases := switchCases(node)
	defaultIndex := -1
	for index, clause := range cases {
		if clause.isDefault {
			defaultIndex = index
		}
	}
	if defaultIndex >= 0 && defaultIndex != len(cases)-1 {
		a.Report(cases[defaultIndex].node, "default clause should be the last switch clause")
	}
}

func switchCases(node cst.Node) []switchCase {
	result := []switchCase{}
	cst.Walk(
		node,
		func(child cst.Node) bool {
			switch clause := child.(type) {
			case *cst.ExprCaseClauseList:
				item := switchCase{
					node: clause,
					body: clause.StatementList,
				}
				if clause.ExprSwitchCase != nil {
					item.isDefault = strings.HasPrefix(cst.Spelling(clause.ExprSwitchCase), "default")
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
				item := switchCase{
					node: clause,
					body: clause.StatementList,
				}
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
	sort.SliceStable(result, func(i, j int) bool {
		left, _ := cst.Range(result[i].node)
		right, _ := cst.Range(result[j].node)
		return left < right
	})
	return result
}

func statementListSpelling(list *cst.StatementList) string {
	statements := statementsFromList(list)
	parts := make([]string, 0, len(statements))
	for _, statement := range statements {
		parts = append(parts, cst.Spelling(statement))
	}
	return strings.Join(parts, "\n")
}

func statementsFromList(list *cst.StatementList) []cst.Node {
	result := []cst.Node{}
	for ; list != nil; list = list.List {
		if list.Statement != nil {
			result = append(result, list.Statement)
		}
	}
	return result
}
