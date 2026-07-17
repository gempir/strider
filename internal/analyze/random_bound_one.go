package analyze

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type randomBoundOneRule struct {}

func (randomBoundOneRule) Meta() Meta {
	return Meta{
		Code: "random-bound-one",
		Summary: "detect random integer calls whose upper bound permits only zero",
		Explanation: "Bounded random integer functions generate values in the half-open range from zero up to, but excluding, the bound. A bound of one therefore always returns zero.",
		GoodExample: "choice := rand.Intn(2)",
		BadExample: "choice := rand.Intn(1)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (randomBoundOneRule) Run(pass *Pass) {
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok || len(call.Common().Args) == 0 {
					continue
				}
				name, ok := boundedRandomFunction(call)
				if !ok {
					continue
				}
				bound := ssaConstant(call.Common().Args[len(call.Common().Args) - 1])
				if bound == nil || bound.Value == nil || bound.Value.Kind() != constant.Int || !constant.Compare(
					bound.Value,
					token.EQL,
					constant.MakeInt64(1),
				) {
					continue
				}
				pass.Report(
					positionNode{position: call.Pos()},
					fmt.Sprintf("%s with an upper bound of one always returns zero", name),
				)
			}
		}
	}
}

func boundedRandomFunction(call ssa.CallInstruction) (string, bool) {
	callee := call.Common().StaticCallee()
	if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil {
		return "", false
	}
	function, ok := callee.Object().(*types.Func)
	if !ok {
		return "", false
	}
	packagePath := function.Pkg().Path()
	name := function.Name()
	switch packagePath {
	case "math/rand":
		switch name {
		case "Int31n", "Int63n", "Intn":
			return "rand." + name, true
		}
	case "math/rand/v2":
		switch name {
		case "Int32N", "Int64N", "IntN", "Uint32N", "Uint64N", "UintN":
			return "rand." + name, true
		case "N":
			signature, _ := function.Type().(*types.Signature)
			if signature != nil && signature.Recv() == nil {
				return "rand.N", true
			}
		}
	}
	return "", false
}
