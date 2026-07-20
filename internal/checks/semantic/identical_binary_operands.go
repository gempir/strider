package semantic

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type identicalBinaryOperandsCheck struct{}

func (identicalBinaryOperandsCheck) Meta() Meta {
	return Meta{
		Code:            "identical-binary-operands",
		Summary:         "detect suspicious binary operations with identical operands",
		Explanation:     "Comparisons and non-idempotent operations with identical expressions on both sides are usually copy-and-paste mistakes. Floating-point expressions are excluded because NaN makes self-comparisons meaningful.",
		GoodExample:     "if left == right { use() }",
		BadExample:      "if left == left { use() }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (identicalBinaryOperandsCheck) Run(pass *Pass) {
	parents := pass.analysisParents()
	pass.Inspect(
		[]ast.Node{
			(*ast.BinaryExpr)(nil),
		},
		func(node ast.Node) bool {
			binary, ok := node.(*ast.BinaryExpr)
			if !ok || !suspiciousSelfOperator(binary.Op) || typeMayContainFloat(pass.TypesInfo.TypeOf(binary.X)) || renderAnalysisExpression(pass, binary.X) != renderAnalysisExpression(
				pass,
				binary.Y,
			) || isRandomSelfCall(pass, binary.X) || isComparableAssertion(binary, parents) {
				return true
			}
			pass.Report(binary, "identical expressions appear on both sides of "+binary.Op.String())
			return true
		},
	)
}

func suspiciousSelfOperator(operator token.Token) bool {
	switch operator {
	case token.EQL, token.NEQ, token.SUB, token.QUO, token.AND, token.REM, token.OR, token.XOR, token.AND_NOT, token.LAND, token.LOR, token.LSS, token.GTR, token.LEQ, token.GEQ:
		return true
	default:
		return false
	}
}

func typeMayContainFloat(valueType types.Type) bool {
	if valueType == nil {
		return true
	}
	valueType = types.Unalias(valueType)
	switch valueType := valueType.Underlying().(type) {
	case *types.Basic:
		return valueType.Kind() == types.Float32 || valueType.Kind() == types.Float64
	case *types.Array:
		return typeMayContainFloat(valueType.Elem())
	case *types.Struct:
		for index := range valueType.NumFields() {
			if typeMayContainFloat(valueType.Field(index).Type()) {
				return true
			}
		}
	}
	return false
}

func isRandomSelfCall(pass *Pass, expression ast.Expr) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return false
	}
	function := calledFunction(pass.TypesInfo, call.Fun)
	return function != nil && function.Pkg() != nil && (function.Pkg().Path() == "math/rand" || function.Pkg().Path() == "math/rand/v2")
}

func isComparableAssertion(binary *ast.BinaryExpr, parents map[ast.Node]ast.Node) bool {
	left, leftOK := binary.X.(*ast.CompositeLit)
	right, rightOK := binary.Y.(*ast.CompositeLit)
	if !leftOK || !rightOK || len(left.Elts) != 0 || len(right.Elts) != 0 {
		return false
	}
	valueSpec, ok := parents[binary].(*ast.ValueSpec)
	if !ok {
		return false
	}
	for index, value := range valueSpec.Values {
		if value == binary && index < len(valueSpec.Names) {
			return valueSpec.Names[index].Name == "_"
		}
	}
	return false
}

func (identicalBinaryOperandsCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
		Facts: FactParents,
	}
}
