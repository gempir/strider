package analyze

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type partiallyTypedConstantGroupRule struct {}

func (partiallyTypedConstantGroupRule) Meta() Meta {
	return Meta{
		Code: "partially-typed-constant-group",
		Summary: "detect constant groups where only the first explicit value has a type",
		Explanation: "In a constant group, an explicit type is inherited only when a later declaration omits its value. If every declaration has an explicit literal but only the first has a type, later constants silently use default built-in types and may lose methods or assignment compatibility.",
		GoodExample: "const ( first Kind = 1; second Kind = 2 )",
		BadExample: "const ( first Kind = 1; second = 2 )",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (partiallyTypedConstantGroupRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		for _, declaration := range file.Decls {
			group, ok := declaration.(*ast.GenDecl)
			if !ok || group.Tok != token.CONST || !group.Lparen.IsValid() || len(group.Specs) < 2 {
				continue
			}
			first, ok := group.Specs[0].(*ast.ValueSpec)
			if !ok || first.Type == nil || len(first.Names) != 1 || len(first.Values) != 1 || !constantLiteral(
				first.Values[0],
			) {
				continue
			}
			firstType := pass.TypesInfo.TypeOf(first.Names[0])
			if firstType == nil || !partiallyTypedLiteralGroup(pass, group.Specs[1:], firstType) {
				continue
			}
			pass.Report(
				first,
				"only the first constant with an explicit value has the declared type; repeat the type on the remaining constants",
			)
		}
	}
}

func partiallyTypedLiteralGroup(pass *Pass, specs []ast.Spec, firstType types.Type) bool {
	for _, raw := range specs {
		specification, ok := raw.(*ast.ValueSpec)
		if !ok || specification.Type != nil || len(specification.Names) != 1 || len(
			specification.Values,
		) != 1 || !constantLiteral(specification.Values[0]) {
			return false
		}
		valueType := pass.TypesInfo.TypeOf(specification.Values[0])
		if valueType == nil || !types.ConvertibleTo(valueType, firstType) {
			return false
		}
	}
	return true
}

func constantLiteral(expression ast.Expr) bool {
	switch expression := expression.(type) {
	case *ast.BasicLit:
		return true
	case *ast.UnaryExpr:
		_, ok := expression.X.(*ast.BasicLit)
		return ok
	default:
		return false
	}
}
