package rules

import (
	"fmt"
	"go/ast"
)

func (a *analyzer) checkUnusedFunctionParts(fn *ast.FuncDecl) {
	if fn.Body == nil {
		return
	}
	uses := identifierUses(fn.Body)
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			for _, name := range field.Names {
				if name.Name != "_" && uses[name.Name] == 0 {
					a.report(
						"unused-parameter",
						name,
						fmt.Sprintf("parameter %s is unused", name.Name),
					)
				}
			}
		}
	}
	if fn.Recv != nil {
		for _, field := range fn.Recv.List {
			for _, name := range field.Names {
				if name.Name != "_" && uses[name.Name] == 0 {
					a.report(
						"unused-receiver",
						name,
						fmt.Sprintf("receiver %s is unused", name.Name),
					)
				}
				if name.Name == "this" || name.Name == "self" {
					a.report(
						"receiver-naming",
						name,
						"receiver name should be a short abbreviation of its type",
					)
				}
			}
		}
	}
	for _, field := range booleanParameters(fn.Type.Params) {
		name := field.Name
		ast.Inspect(
			fn.Body,
			func(node ast.Node) bool {
				statement, ok := node.(*ast.IfStmt)
				if ok && expressionContainsIdent(statement.Cond, name) {
					a.report(
						"flag-parameter",
						field,
						fmt.Sprintf("boolean parameter %s controls function flow", name),
					)
					return false
				}
				return true
			},
		)
	}
}

func identifierUses(node ast.Node) map[string]int {
	uses := map[string]int{}
	ast.Inspect(
		node,
		func(child ast.Node) bool {
			if id, ok := child.(*ast.Ident); ok {
				uses[id.Name]++
			}
			return true
		},
	)
	return uses
}

func booleanParameters(list *ast.FieldList) []*ast.Ident {
	var names []*ast.Ident
	if list == nil {
		return names
	}
	for _, field := range list.List {
		if exprText(field.Type) == "bool" {
			names = append(names, field.Names...)
		}
	}
	return names
}
