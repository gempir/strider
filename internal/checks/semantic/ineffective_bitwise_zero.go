package semantic

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type ineffectiveBitwiseZeroRule struct{}

func (ineffectiveBitwiseZeroRule) Meta() Meta {
	return Meta{
		Code:            "ineffective-bitwise-zero",
		Summary:         "detect bitwise operations whose zero operand fixes the result",
		Explanation:     "For integers, x & 0 is always zero while x | 0 and x ^ 0 are always x. A zero-valued flag declared directly with iota often indicates that 1 << iota was intended.",
		GoodExample:     "masked := value & mask",
		BadExample:      "unchanged := value ^ 0",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (ineffectiveBitwiseZeroRule) Run(pass *Pass) {
	iotaConstants := directIotaConstants(pass)
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				binary,
					ok := node.(*ast.BinaryExpr)
				if !ok || !allIntegerTypes(pass.TypesInfo.TypeOf(binary)) {
					return true
				}
				switch binary.Op {
				case token.AND,
					token.OR,
					token.XOR:
				default:
					return true
				}
				zero,
					iotaName := bitwiseZeroOperand(pass, binary.Y, iotaConstants)
				if !zero {
					return true
				}
				expression := renderAnalysisExpression(pass, binary)
				left := renderAnalysisExpression(pass, binary.X)
				var message string
				switch binary.Op {
				case token.AND:
					message = fmt.Sprintf("%s always equals zero", expression)
				case token.OR,
					token.XOR:
					message = fmt.Sprintf("%s always equals %s", expression, left)
				}
				if iotaName != "" {
					message += fmt.Sprintf("; %s is zero because it is declared with iota—did you mean 1 << iota?", iotaName)
				}
				pass.Report(binary, message)
				return true
			},
		)
	}
}

func directIotaConstants(pass *Pass) map[*types.Const]bool {
	constants := make(map[*types.Const]bool)
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				specification,
					ok := node.(*ast.ValueSpec)
				if !ok || len(specification.Names) != 1 || len(specification.Values) != 1 {
					return true
				}
				identifier,
					ok := ast.Unparen(specification.Values[0]).(*ast.Ident)
				if !ok || !isUniverseIota(pass.TypesInfo.ObjectOf(identifier)) {
					return true
				}
				object,
					ok := pass.TypesInfo.Defs[specification.Names[0]].(*types.Const)
				if ok {
					constants[object] = true
				}
				return true
			},
		)
	}
	return constants
}

func isUniverseIota(object types.Object) bool {
	constantObject, ok := object.(*types.Const)
	return ok && constantObject.Pkg() == nil && constantObject.Name() == "iota"
}

func bitwiseZeroOperand(pass *Pass, expression ast.Expr, iotaConstants map[*types.Const]bool) (bool, string) {
	unwrapped := ast.Unparen(expression)
	if literal, ok := unwrapped.(*ast.BasicLit); ok && literal.Kind == token.INT {
		value := pass.TypesInfo.Types[expression].Value
		if value == nil {
			value = pass.TypesInfo.Types[unwrapped].Value
		}
		return value != nil && constant.Sign(value) == 0, ""
	}
	identifier, ok := unwrapped.(*ast.Ident)
	if !ok {
		return false, ""
	}
	object, ok := pass.TypesInfo.ObjectOf(identifier).(*types.Const)
	if !ok || object.Pkg() != pass.Types || !iotaConstants[object] || constant.Sign(object.Val()) != 0 {
		return false, ""
	}
	return true, identifier.Name
}
