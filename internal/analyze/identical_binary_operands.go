package analyze

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type identicalBinaryOperandsRule struct {}

func (identicalBinaryOperandsRule) Meta() Meta {
	return Meta{
		Code: "identical-binary-operands",
		Summary: "detect suspicious binary operations with identical operands",
		Explanation: "Comparisons and non-idempotent operations with identical expressions on both sides are usually copy-and-paste mistakes. Floating-point expressions are excluded because NaN makes self-comparisons meaningful.",
		GoodExample: "if left == right { use() }",
		BadExample: "if left == left { use() }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (identicalBinaryOperandsRule) Run(pass *Pass) {
	parents := pass.analysisParents()
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				binary,
				ok := node.(*ast.BinaryExpr)
				if !ok || !suspiciousSelfOperator(binary.Op) || typeMayContainFloat(
					pass.TypesInfo.TypeOf(binary.X),
				) || renderAnalysisExpression(pass, binary.X) != renderAnalysisExpression(
					pass,
					binary.Y,
				) || isRandomSelfCall(pass, binary.X) || isComparableAssertion(binary, parents) {
					return true
				}
				pass.Report(
					binary,
					"identical expressions appear on both sides of " + binary.Op.String(),
				)
				return true
			},
		)
	}
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

func (pass *Pass) analysisParents() map[ast.Node]ast.Node {
	pass.facts.parentsOnce.Do(
		func() {
			pass.facts.parents = make(map[ast.Node]ast.Node)
			for _,
			file := range pass.Files {
				stack := make([]ast.Node, 0)
				ast.Inspect(
					file,
					func(node ast.Node) bool {
						if node == nil {
							if len(stack) != 0 {
								stack = stack[:len(stack) - 1]
							}
							return true
						}
						if len(stack) != 0 {
							pass.facts.parents[node] = stack[len(stack) - 1]
						}
						stack = append(stack, node)
						return true
					},
				)
			}
		},
	)
	return pass.facts.parents
}
