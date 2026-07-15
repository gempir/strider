package rules

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

func (a *analyzer) checkFunction(fn *ast.FuncDecl) {
	a.checkCyclomaticComplexity(fn)
	a.checkMaxParameters(fn)
	a.checkNoInit(fn)
	parameters := fieldCount(fn.Type.Params)
	if parameters > 8 {
		a.report(
			"argument-limit",
			fn.Name,
			fmt.Sprintf("function has %d parameters; maximum is 8", parameters),
		)
	}
	if fn.Type.Results != nil && fieldCount(fn.Type.Results) > 3 {
		a.report(
			"function-result-limit",
			fn.Name,
			fmt.Sprintf("function returns %d values; maximum is 3", fieldCount(fn.Type.Results)),
		)
	}
	complexity := functionComplexity(fn.Body)
	if complexity > 10 {
		a.report(
			"cyclomatic",
			fn.Name,
			fmt.Sprintf("function has cyclomatic complexity %d; maximum is 10", complexity),
		)
	}
	cognitive := cognitiveComplexity(fn.Body)
	if cognitive > 7 {
		a.report(
			"cognitive-complexity",
			fn.Name,
			fmt.Sprintf("function has cognitive complexity %d; maximum is 7", cognitive),
		)
	}
	if fn.Body != nil {
		statements := statementCount(fn.Body)
		lines := a.fset.Position(fn.Body.End()).Line - a.fset.Position(fn.Body.Pos()).Line + 1
		if statements > 50 || lines > 75 {
			a.report(
				"function-length",
				fn.Name,
				fmt.Sprintf(
					"function has %d statements and %d lines; maximum is 50 statements or 75 lines",
					statements,
					lines,
				),
			)
		}
		if fn.Type.Results == nil && len(fn.Body.List) > 0 {
			if ret, ok := fn.Body.List[len(fn.Body.List)-1].(*ast.ReturnStmt); ok &&
				len(ret.Results) == 0 {
				a.report(
					"unnecessary-stmt",
					ret,
					"omit the unnecessary return at the end of a resultless function",
				)
			}
		}
	}
	if strings.HasPrefix(strings.ToUpper(fn.Name.Name), "GET") && fieldCount(fn.Type.Results) == 0 {
		a.report("get-return", fn.Name, "Get-prefixed function should return a value")
	}
	a.checkExportedFunction(fn)
	a.checkFunctionResults(fn)
	a.checkContextArgument(fn)
	a.checkFunctionNames(fn)
	a.checkUnusedFunctionParts(fn)
	a.checkFunctionMutation(fn)
	a.checkFunctionControl(fn)
}

func functionComplexity(body *ast.BlockStmt) int {
	if body == nil {
		return 0
	}
	complexity := 1
	ast.Inspect(
		body,
		func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.TypeSwitchStmt:
				complexity++
			case *ast.CaseClause:
				if len(n.List) != 0 {
					complexity++
				}
			case *ast.CommClause:
				if n.Comm != nil {
					complexity++
				}
			case *ast.BinaryExpr:
				if n.Op == token.LAND || n.Op == token.LOR {
					complexity++
				}
			}
			return true
		},
	)
	return complexity
}

func cognitiveComplexity(body *ast.BlockStmt) int {
	if body == nil {
		return 0
	}
	total := 0
	var visit func(ast.Node, int)
	visit = func(node ast.Node, nesting int) {
		if node == nil {
			return
		}
		next := nesting
		switch node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			total += 1 + nesting
			next++
		case *ast.BranchStmt:
			total++
		}
		ast.Inspect(
			node,
			func(child ast.Node) bool {
				if child == nil || child == node {
					return child != nil
				}
				visit(child, next)
				return false
			},
		)
	}
	visit(body, 0)
	return total
}

func statementCount(body *ast.BlockStmt) int {
	count := 0
	ast.Inspect(
		body,
		func(node ast.Node) bool {
			if _, ok := node.(ast.Stmt); ok {
				count++
			}
			return true
		},
	)
	return max(0, count-1)
}
