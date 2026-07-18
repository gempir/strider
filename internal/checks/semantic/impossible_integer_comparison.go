package semantic

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type impossibleIntegerComparisonRule struct{}

func (impossibleIntegerComparisonRule) Meta() Meta {
	return Meta{
		Code:            "impossible-integer-comparison",
		Summary:         "detect integer comparisons that are fixed by the type's range",
		Explanation:     "An integer type's minimum and maximum values make some comparisons always true or false, such as checking whether an unsigned value is below zero or above its maximum.",
		GoodExample:     "if value == 0 { use() }",
		BadExample:      "if value < 0 { use() } // value is unsigned",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (impossibleIntegerComparisonRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				binary,
					ok := node.(*ast.BinaryExpr)
				if !ok || !relationalOperator(binary.Op) {
					return true
				}
				value,
					bound,
					operator,
					ok := integerComparisonParts(pass, binary)
				if !ok {
					return true
				}
				minimum,
					maximum,
					ok := integerTypeBounds(pass.TypesInfo.TypeOf(value), pass.TypesSizes)
				if !ok {
					return true
				}
				fixed,
					truth := comparisonFixedByBounds(operator, bound, minimum, maximum)
				if fixed {
					pass.Report(
						binary,
						fmt.Sprintf(
							"comparison is always %t for type %s",
							truth,
							pass.TypesInfo.TypeOf(value),
						),
					)
				}
				return true
			},
		)
	}
}

func relationalOperator(operator token.Token) bool {
	switch operator {
	case token.LSS, token.LEQ, token.GTR, token.GEQ:
		return true
	default:
		return false
	}
}

func integerComparisonParts(pass *Pass, binary *ast.BinaryExpr) (
	ast.Expr,
	constant.Value,
	token.Token,
	bool,
) {
	if bound := pass.TypesInfo.Types[binary.Y].Value; bound != nil && bound.Kind() == constant.Int {
		return binary.X, bound, binary.Op, true
	}
	if bound := pass.TypesInfo.Types[binary.X].Value; bound != nil && bound.Kind() == constant.Int {
		return binary.Y, bound, reverseComparison(binary.Op), true
	}
	return nil, nil, token.ILLEGAL, false
}

func reverseComparison(operator token.Token) token.Token {
	switch operator {
	case token.LSS:
		return token.GTR
	case token.LEQ:
		return token.GEQ
	case token.GTR:
		return token.LSS
	case token.GEQ:
		return token.LEQ
	default:
		return operator
	}
}

func integerTypeBounds(valueType types.Type, sizes types.Sizes) (
	constant.Value,
	constant.Value,
	bool,
) {
	if valueType == nil {
		return nil, nil, false
	}
	basic, ok := types.Unalias(valueType).Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsInteger == 0 {
		return nil, nil, false
	}
	bits := int64(0)
	signed := basic.Info()&types.IsUnsigned == 0
	switch basic.Kind() {
	case types.Int8, types.Uint8:
		bits = 8
	case types.Int16, types.Uint16:
		bits = 16
	case types.Int32, types.Uint32:
		bits = 32
	case types.Int64, types.Uint64:
		bits = 64
	case types.Int, types.Uint, types.Uintptr:
		if sizes == nil {
			return nil, nil, false
		}
		bits = sizes.Sizeof(valueType) * 8
	default:
		return nil, nil, false
	}
	one := constant.MakeInt64(1)
	if !signed {
		maximum := constant.BinaryOp(constant.Shift(one, token.SHL, uint(bits)), token.SUB, one)
		return constant.MakeInt64(0), maximum, true
	}
	limit := constant.Shift(one, token.SHL, uint(bits-1))
	minimum := constant.UnaryOp(token.SUB, limit, 0)
	maximum := constant.BinaryOp(limit, token.SUB, one)
	return minimum, maximum, true
}

func comparisonFixedByBounds(operator token.Token, bound, minimum, maximum constant.Value) (
	bool,
	bool,
) {
	comparedToMinimum := constant.Sign(constant.BinaryOp(bound, token.SUB, minimum))
	comparedToMaximum := constant.Sign(constant.BinaryOp(bound, token.SUB, maximum))
	switch operator {
	case token.LSS:
		if comparedToMinimum <= 0 {
			return true, false
		}
		if comparedToMaximum > 0 {
			return true, true
		}
	case token.LEQ:
		if comparedToMinimum < 0 {
			return true, false
		}
		if comparedToMaximum >= 0 {
			return true, true
		}
	case token.GTR:
		if comparedToMaximum >= 0 {
			return true, false
		}
		if comparedToMinimum < 0 {
			return true, true
		}
	case token.GEQ:
		if comparedToMaximum > 0 {
			return true, false
		}
		if comparedToMinimum <= 0 {
			return true, true
		}
	}
	return false, false
}
