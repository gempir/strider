package semantic

import (
	"go/token"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type nanComparisonRule struct{}

func (nanComparisonRule) Meta() Meta {
	return Meta{
		Code:            "nan-comparison",
		Summary:         "detect direct comparisons with NaN",
		Explanation:     "IEEE floating-point NaN is unequal to every value, including itself, and all ordered comparisons with it are false. Use math.IsNaN when testing whether a value is NaN.",
		GoodExample:     "if math.IsNaN(value) { handle() }",
		BadExample:      "if value == math.NaN() { handle() }",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (nanComparisonRule) Run(pass *Pass) {
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				binary, ok := instruction.(*ssa.BinOp)
				if !ok || !comparisonOperator(binary.Op) || (!isNaNValue(
					flattenEquivalentPhi(binary.X),
				) && !isNaNValue(flattenEquivalentPhi(binary.Y))) {
					continue
				}
				pass.Report(
					positionNode{position: binary.Pos()},
					"direct comparison with NaN can never test for NaN; use math.IsNaN",
				)
			}
		}
	}
}

func comparisonOperator(operator token.Token) bool {
	switch operator {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		return true
	default:
		return false
	}
}

func isNaNValue(value ssa.Value) bool {
	call, ok := value.(*ssa.Call)
	return ok && isStaticFunction(call, "math", "NaN")
}

func flattenEquivalentPhi(value ssa.Value) ssa.Value {
	seen := make(map[ssa.Value]bool)
	var result ssa.Value
	failed := false
	var visit func(ssa.Value)
	visit = func(current ssa.Value) {
		if current == nil || failed || seen[current] {
			return
		}
		seen[current] = true
		if phi, ok := current.(*ssa.Phi); ok {
			for _, edge := range phi.Edges {
				visit(edge)
			}
			return
		}
		if result == nil {
			result = current
		} else if result != current {
			failed = true
		}
	}
	visit(value)
	if failed {
		return nil
	}
	return result
}
