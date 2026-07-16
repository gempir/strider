package analyze

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type sortConversionWithoutSortRule struct{}

func (sortConversionWithoutSortRule) Meta() Meta {
	return Meta{
		Code:            "sort-conversion-without-sort",
		Summary:         "detect slice type conversions mistaken for sorting calls",
		Explanation:     "sort.Float64Slice, sort.IntSlice, and sort.StringSlice are types, not sorting functions. Converting a slice to one of these types and assigning it back does not reorder any values.",
		GoodExample:     "sort.Ints(values)",
		BadExample:      "values = sort.IntSlice(values)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (sortConversionWithoutSortRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			assignment, ok := node.(*ast.AssignStmt)
			if !ok || assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 ||
				len(assignment.Rhs) != 1 {
				return true
			}
			target, ok := assignment.Lhs[0].(*ast.Ident)
			if !ok {
				return true
			}
			if _, plainSlice := types.Unalias(pass.TypesInfo.TypeOf(target)).(*types.Slice); !plainSlice {
				return true
			}
			conversion, ok := ast.Unparen(assignment.Rhs[0]).(*ast.CallExpr)
			if !ok || len(conversion.Args) != 1 {
				return true
			}
			argument, ok := ast.Unparen(conversion.Args[0]).(*ast.Ident)
			if !ok || pass.TypesInfo.ObjectOf(argument) != pass.TypesInfo.ObjectOf(target) {
				return true
			}
			typeName := sortSliceTypeName(pass, conversion.Fun)
			if typeName == "" {
				return true
			}
			helper := map[string]string{
				"Float64Slice": "Float64s",
				"IntSlice":     "Ints",
				"StringSlice":  "Strings",
			}[typeName]
			pass.Report(
				assignment,
				fmt.Sprintf("sort.%s is only a type conversion and does not sort; use sort.%s", typeName, helper),
			)
			return true
		})
	}
}

func sortSliceTypeName(pass *Pass, expression ast.Expr) string {
	selector, ok := ast.Unparen(expression).(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	typeName, ok := pass.TypesInfo.ObjectOf(selector.Sel).(*types.TypeName)
	if !ok || typeName.Pkg() == nil || typeName.Pkg().Path() != "sort" {
		return ""
	}
	switch typeName.Name() {
	case "Float64Slice", "IntSlice", "StringSlice":
		return typeName.Name()
	default:
		return ""
	}
}
