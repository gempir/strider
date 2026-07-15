package rules

import (
	"go/ast"
	"go/token"
	"strings"
)

func (a *analyzer) checkSwitch(body *ast.BlockStmt) {
	if body == nil {
		return
	}
	if len(body.List) == 1 {
		if clause, ok := body.List[0].(*ast.CaseClause); ok && len(clause.List) <= 1 {
			a.report(
				"unnecessary-stmt",
				body,
				"switch with one case can be replaced by an if statement",
			)
		}
	}
	conditions := map[string]ast.Node{}
	branches := map[string]ast.Node{}
	defaultIndex := -1
	for index, raw := range body.List {
		clause, ok := raw.(*ast.CaseClause)
		if !ok {
			continue
		}
		if len(clause.List) == 0 {
			defaultIndex = index
		}
		for _, condition := range clause.List {
			text := nodeText(condition)
			if conditions[text] != nil {
				a.report(
					"identical-switch-conditions",
					condition,
					"switch repeats a case condition",
				)
			} else {
				conditions[text] = condition
			}
		}
		text := statementsText(clause.Body)
		if text != "" {
			if branches[text] != nil {
				a.report("identical-switch-branches", clause, "switch repeats a case body")
			} else {
				branches[text] = clause
			}
		}
		if len(clause.Body) > 0 {
			if branch, ok := clause.Body[len(clause.Body)-1].(*ast.BranchStmt); ok &&
				branch.Tok == token.FALLTHROUGH &&
				index == len(body.List)-1 {
				a.report(
					"useless-fallthrough",
					branch,
					"fallthrough in the final switch case has no effect",
				)
			}
		}
	}
	if defaultIndex >= 0 && defaultIndex != len(body.List)-1 {
		a.report(
			"enforce-switch-style",
			body.List[defaultIndex],
			"default clause should be the last switch clause",
		)
	}
}

func statementsText(statements []ast.Stmt) string {
	var parts []string
	for _, statement := range statements {
		parts = append(parts, nodeText(statement))
	}
	return strings.Join(parts, "\n")
}
