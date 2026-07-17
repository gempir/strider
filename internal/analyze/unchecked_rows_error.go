package analyze

import (
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type uncheckedRowsErrorRule struct {}

func (uncheckedRowsErrorRule) Meta() Meta {
	return Meta{
		Code: "unchecked-rows-error",
		Summary: "detect sql.Rows iteration without an Err check",
		Explanation: "Rows.Next returns false both at the end of a result set and when iteration fails. Check Rows.Err after iteration to distinguish successful completion from a driver, network, or decoding failure.",
		GoodExample: "for rows.Next() { scan(rows) }; if err := rows.Err(); err != nil { return err }",
		BadExample: "for rows.Next() { scan(rows) }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (uncheckedRowsErrorRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				var body *ast.BlockStmt
				switch function := node.(type) {
				case *ast.FuncDecl:
					body = function.Body
				case *ast.FuncLit:
					body = function.Body
				default:
					return true
				}
				if body != nil {
					reportUncheckedRowsErrors(pass, body)
				}
				return true
			},
		)
	}
}

func reportUncheckedRowsErrors(pass *Pass, body *ast.BlockStmt) {
	iterated := make(map[types.Object]ast.Node)
	checked := make(map[types.Object]bool)
	ast.Inspect(
		body,
		func(node ast.Node) bool {
			if _,
			nested := node.(*ast.FuncLit); nested {
				return false
			}
			call,
			ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector,
			ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !isSQLRowsType(pass.TypesInfo.TypeOf(selector.X)) {
				return true
			}
			receiver,
			ok := unparenExpression(selector.X).(*ast.Ident)
			if !ok {
				return true
			}
			object := pass.TypesInfo.ObjectOf(receiver)
			if object == nil {
				return true
			}
			switch selector.Sel.Name {
			case "Next",
				"NextResultSet":
				if iterated[object] == nil {
					iterated[object] = call
				}
			case "Err":
				checked[object] = true
			}
			return true
		},
	)
	for object, node := range iterated {
		if checked[object] {
			continue
		}
		pass.Report(
			node,
			"sql.Rows iteration does not check Rows.Err; iteration failures are indistinguishable from successful completion",
		)
	}
}

func isSQLRowsType(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	pointer, ok := types.Unalias(valueType).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(pointer.Elem()).(*types.Named)
	return ok && named.Obj() != nil && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "database/sql" && named.Obj().Name() == "Rows"
}
