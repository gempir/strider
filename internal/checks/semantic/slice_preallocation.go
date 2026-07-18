package semantic

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type slicePreallocationRule struct{}

func (slicePreallocationRule) Meta() Meta {
	return Meta{
		Code:            "slice-preallocation",
		Summary:         "detect slices that can use range-source capacity",
		Explanation:     "A slice grown once per iteration of a range over a slice, array, map, or string has a useful capacity bound. Initializing it as make([]T, 0, len(source)) avoids repeated growth and copying while preserving its zero length.",
		GoodExample:     "result := make([]Item, 0, len(source))\nfor _, item := range source { result = append(result, convert(item)) }",
		BadExample:      "var result []Item\nfor _, item := range source { result = append(result, convert(item)) }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

type emptySliceCandidate struct {
	identifier *ast.Ident
	variable   *types.Var
}

func (slicePreallocationRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				block,
					ok := node.(*ast.BlockStmt)
				if !ok {
					return true
				}
				checkPreallocationBlock(pass, block)
				return true
			},
		)
	}
}

func checkPreallocationBlock(pass *Pass, block *ast.BlockStmt) {
	candidates := make(map[*types.Var]emptySliceCandidate)
	for _, statement := range block.List {
		newCandidates := declaredEmptySlices(pass, statement)
		if len(newCandidates) != 0 {
			for variable := range candidates {
				if statementMutatesSlice(pass, statement, variable) {
					delete(candidates, variable)
				}
			}
			for _, candidate := range newCandidates {
				candidates[candidate.variable] = candidate
			}
			continue
		}

		if loop, ok := statement.(*ast.RangeStmt); ok {
			if rangeHasUsefulLength(pass.TypesInfo.TypeOf(loop.X)) {
				for variable, candidate := range candidates {
					if !rangeSourceIsVariable(pass, loop.X, variable) && rangeAppendsExactlyOnce(
						pass,
						loop,
						variable,
					) {
						pass.Report(
							candidate.identifier,
							fmt.Sprintf(
								"preallocate %s with capacity len(range source) before appending once per iteration",
								candidate.identifier.Name,
							),
						)
						delete(candidates, variable)
					}
				}
			}
		}

		for variable := range candidates {
			if statementMutatesSlice(pass, statement, variable) {
				delete(candidates, variable)
			}
		}
	}
}

func declaredEmptySlices(pass *Pass, statement ast.Stmt) []emptySliceCandidate {
	var result []emptySliceCandidate
	switch statement := statement.(type) {
	case *ast.DeclStmt:
		declaration, ok := statement.Decl.(*ast.GenDecl)
		if !ok || declaration.Tok != token.VAR {
			return nil
		}
		for _, specification := range declaration.Specs {
			value, ok := specification.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, name := range value.Names {
				variable, ok := pass.TypesInfo.Defs[name].(*types.Var)
				if !ok || !isSliceType(variable.Type()) {
					continue
				}
				if len(value.Values) == 0 || (index < len(value.Values) && emptySliceExpression(
					pass,
					value.Values[index],
				)) {
					result = append(result, emptySliceCandidate{identifier: name, variable: variable})
				}
			}
		}
	case *ast.AssignStmt:
		if statement.Tok != token.DEFINE {
			return nil
		}
		for index, left := range statement.Lhs {
			if index >= len(statement.Rhs) || !emptySliceExpression(pass, statement.Rhs[index]) {
				continue
			}
			name, ok := left.(*ast.Ident)
			if !ok {
				continue
			}
			variable, ok := pass.TypesInfo.Defs[name].(*types.Var)
			if ok && isSliceType(variable.Type()) {
				result = append(result, emptySliceCandidate{identifier: name, variable: variable})
			}
		}
	}
	return result
}

func emptySliceExpression(pass *Pass, expression ast.Expr) bool {
	switch expression := expression.(type) {
	case *ast.CompositeLit:
		return len(expression.Elts) == 0 && isSliceType(pass.TypesInfo.TypeOf(expression))
	case *ast.CallExpr:
		if len(expression.Args) != 2 || !isBuiltin(pass.TypesInfo, expression.Fun, "make") || !isSliceType(
			pass.TypesInfo.TypeOf(expression),
		) {
			return false
		}
		length := pass.TypesInfo.Types[expression.Args[1]].Value
		return length != nil && length.Kind() == constant.Int && constant.Sign(length) == 0
	default:
		return false
	}
}

func isSliceType(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	_, ok := types.Unalias(valueType).Underlying().(*types.Slice)
	return ok
}

func rangeHasUsefulLength(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	underlying := types.Unalias(valueType).Underlying()
	switch underlying := underlying.(type) {
	case *types.Array, *types.Slice, *types.Map:
		return true
	case *types.Pointer:
		_, ok := types.Unalias(underlying.Elem()).Underlying().(*types.Array)
		return ok
	case *types.Basic:
		return underlying.Info()&types.IsString != 0
	default:
		return false
	}
}

func rangeAppendsExactlyOnce(pass *Pass, loop *ast.RangeStmt, variable *types.Var) bool {
	direct := 0
	for _, statement := range loop.Body.List {
		if appendedSlice(pass, statement) == variable {
			direct++
		}
	}
	if direct != 1 {
		return false
	}
	all := 0
	assignments := 0
	addressTaken := false
	ast.Inspect(
		loop.Body,
		func(node ast.Node) bool {
			if _,
				nested := node.(*ast.FuncLit); nested {
				return false
			}
			switch node := node.(type) {
			case *ast.CallExpr:
				if appendTarget(pass, node) == variable {
					all++
				}
			case *ast.AssignStmt:
				for _, left := range node.Lhs {
					identifier,
						ok := left.(*ast.Ident)
					if ok && pass.TypesInfo.ObjectOf(identifier) == variable {
						assignments++
					}
				}
			case *ast.UnaryExpr:
				identifier,
					ok := node.X.(*ast.Ident)
				if node.Op == token.AND && ok && pass.TypesInfo.ObjectOf(identifier) == variable {
					addressTaken = true
				}
			}
			return true
		},
	)
	return all == 1 && assignments == 1 && !addressTaken
}

func rangeSourceIsVariable(pass *Pass, expression ast.Expr, variable *types.Var) bool {
	switch expression := expression.(type) {
	case *ast.Ident:
		return pass.TypesInfo.ObjectOf(expression) == variable
	case *ast.ParenExpr:
		return rangeSourceIsVariable(pass, expression.X, variable)
	case *ast.SliceExpr:
		return rangeSourceIsVariable(pass, expression.X, variable)
	default:
		return false
	}
}

func appendedSlice(pass *Pass, statement ast.Stmt) *types.Var {
	assignment, ok := statement.(*ast.AssignStmt)
	if !ok || assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return nil
	}
	left, ok := assignment.Lhs[0].(*ast.Ident)
	if !ok {
		return nil
	}
	call, ok := assignment.Rhs[0].(*ast.CallExpr)
	if !ok || call.Ellipsis.IsValid() || len(call.Args) != 2 {
		return nil
	}
	target := appendTarget(pass, call)
	variable, _ := pass.TypesInfo.ObjectOf(left).(*types.Var)
	if variable == nil || variable != target {
		return nil
	}
	return variable
}

func appendTarget(pass *Pass, call *ast.CallExpr) *types.Var {
	if !isBuiltin(pass.TypesInfo, call.Fun, "append") || len(call.Args) == 0 {
		return nil
	}
	identifier, ok := call.Args[0].(*ast.Ident)
	if !ok {
		return nil
	}
	variable, _ := pass.TypesInfo.ObjectOf(identifier).(*types.Var)
	return variable
}

func statementMutatesSlice(pass *Pass, statement ast.Stmt, variable *types.Var) bool {
	mutated := false
	ast.Inspect(
		statement,
		func(node ast.Node) bool {
			if mutated || node == nil {
				return !mutated
			}
			switch node := node.(type) {
			case *ast.AssignStmt:
				for _, left := range node.Lhs {
					identifier,
						ok := left.(*ast.Ident)
					if ok && pass.TypesInfo.ObjectOf(identifier) == variable {
						mutated = true
						return false
					}
				}
			case *ast.CallExpr:
				if appendTarget(pass, node) == variable {
					mutated = true
					return false
				}
			case *ast.UnaryExpr:
				identifier,
					ok := node.X.(*ast.Ident)
				if node.Op == token.AND && ok && pass.TypesInfo.ObjectOf(identifier) == variable {
					mutated = true
					return false
				}
			}
			return true
		},
	)
	return mutated
}
