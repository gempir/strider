package semantic

import (
	"fmt"
	"go/constant"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type regexpMatchInLoopCheck struct{}

func (regexpMatchInLoopCheck) Meta() Meta {
	return Meta{
		Code:            "regexp-match-in-loop",
		Summary:         "detect repeated regexp compilation inside loops",
		Explanation:     "The package-level regexp matching helpers compile their pattern on every call. Calling them with a constant pattern inside a loop repeats the same compilation; compile the expression once before the loop and reuse it.",
		GoodExample:     "pattern := regexp.MustCompile(`^[a-z]+$`); for _, value := range values { pattern.MatchString(value) }",
		BadExample:      "for _, value := range values { regexp.MatchString(`^[a-z]+$`, value) }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (regexpMatchInLoopCheck) Run(pass *Pass) {
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			if !ssaBlockInCycle(block) {
				continue
			}
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok || len(call.Common().Args) == 0 {
					continue
				}
				name := regexpPackageMatchName(call)
				if name == "" || !ssaStringConstant(call.Common().Args[0]) {
					continue
				}
				pass.ReportPos(call.Pos(), fmt.Sprintf("regexp.%s recompiles a constant pattern on every loop iteration; compile it once before the loop", name))
			}
		}
	}
}

func regexpPackageMatchName(call ssa.CallInstruction) string {
	callee := call.Common().StaticCallee()
	if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil || callee.Object().Pkg().Path() != "regexp" {
		return ""
	}
	switch callee.Object().Name() {
	case "Match", "MatchReader", "MatchString":
		return callee.Object().Name()
	default:
		return ""
	}
}

func ssaStringConstant(value ssa.Value) bool {
	constantValue, ok := unwrapSSAValue(value).(*ssa.Const)
	return ok && constantValue.Value != nil && constantValue.Value.Kind() == constant.String
}

func ssaBlockInCycle(start *ssa.BasicBlock) bool {
	if start == nil {
		return false
	}
	seen := make(map[*ssa.BasicBlock]bool)
	stack := append([]*ssa.BasicBlock(nil), start.Succs...)
	for len(stack) != 0 {
		block := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if block == start {
			return true
		}
		if block == nil || seen[block] {
			continue
		}
		seen[block] = true
		stack = append(stack, block.Succs...)
	}
	return false
}
