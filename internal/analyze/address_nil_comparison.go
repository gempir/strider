package analyze

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type addressNilComparisonRule struct{}

func (addressNilComparisonRule) Meta() Meta {
	return Meta{
		Code:            "address-nil-comparison",
		Summary:         "detect comparisons between a freshly taken address and nil",
		Explanation:     "Taking the address of an addressable value produces a non-nil pointer whenever evaluation completes, so comparing that address with nil has a fixed result.",
		GoodExample:     "if pointer == nil { handle() }",
		BadExample:      "if &value == nil { handle() }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (addressNilComparisonRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			binary, ok := node.(*ast.BinaryExpr)
			if !ok || binary.Op != token.EQL && binary.Op != token.NEQ {
				return true
			}
			if !addressComparedWithNil(pass, binary.X, binary.Y) &&
				!addressComparedWithNil(pass, binary.Y, binary.X) {
				return true
			}
			result := "false"
			if binary.Op == token.NEQ {
				result = "true"
			}
			pass.Report(binary, "address cannot be nil; this comparison is always "+result)
			return true
		})
	}
}

func addressComparedWithNil(pass *Pass, addressExpression, nilExpression ast.Expr) bool {
	address, ok := ast.Unparen(addressExpression).(*ast.UnaryExpr)
	if !ok || address.Op != token.AND {
		return false
	}
	if _, dereference := ast.Unparen(address.X).(*ast.StarExpr); dereference {
		return false
	}
	identifier, ok := ast.Unparen(nilExpression).(*ast.Ident)
	if !ok {
		return false
	}
	_, isNil := pass.TypesInfo.ObjectOf(identifier).(*types.Nil)
	return isNil
}
