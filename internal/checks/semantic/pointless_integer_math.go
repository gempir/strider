package semantic

import (
	"fmt"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type pointlessIntegerMathRule struct{}

func (pointlessIntegerMathRule) Meta() Meta {
	return Meta{
		Code:            "pointless-integer-math",
		Summary:         "detect floating-point helpers applied to converted integers",
		Explanation:     "An integer converted to floating point is already integral and finite. Rounding it or asking whether it is NaN or infinite cannot provide useful information.",
		GoodExample:     "rounded := math.Ceil(measurement)",
		BadExample:      "rounded := math.Ceil(float64(count))",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (pointlessIntegerMathRule) Run(pass *Pass) {
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok || len(call.Common().Args) == 0 {
					continue
				}
				name, ok := pointlessMathFunction(call)
				if !ok {
					continue
				}
				conversion, ok := call.Common().Args[0].(*ssa.Convert)
				if !ok || !allIntegerTypes(conversion.X.Type()) {
					continue
				}
				pass.Report(positionNode{
					position: call.Pos(),
				}, fmt.Sprintf("calling math.%s on a converted integer is pointless", name))
			}
		}
	}
}

func pointlessMathFunction(call ssa.CallInstruction) (string, bool) {
	callee := call.Common().StaticCallee()
	if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil || callee.Object().Pkg().Path() != "math" {
		return "", false
	}
	name := callee.Object().Name()
	switch name {
	case "Ceil", "Floor", "Trunc", "IsNaN", "IsInf":
		return name, true
	default:
		return "", false
	}
}

func allIntegerTypes(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	valueType = types.Unalias(valueType)
	if parameter, ok := valueType.(*types.TypeParam); ok {
		return allIntegerTypes(parameter.Constraint())
	}
	switch underlying := valueType.Underlying().(type) {
	case *types.Basic:
		return underlying.Info()&types.IsInteger != 0
	case *types.Interface:
		if underlying.NumEmbeddeds() == 0 {
			return false
		}
		for index := range underlying.NumEmbeddeds() {
			if !allIntegerTypes(underlying.EmbeddedType(index)) {
				return false
			}
		}
		return true
	case *types.Union:
		for index := range underlying.Len() {
			if !allIntegerTypes(underlying.Term(index).Type()) {
				return false
			}
		}
		return underlying.Len() != 0
	default:
		return false
	}
}
