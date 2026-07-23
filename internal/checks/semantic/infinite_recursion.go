package semantic

import (
	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type infiniteRecursionCheck struct{}

func (infiniteRecursionCheck) Meta() Meta {
	return Meta{
		Code:            "infinite-recursion",
		Summary:         "detect recursive calls with no path to a function exit",
		Explanation:     "A recursive call must have a path that reaches a function exit without making that call. Otherwise recursion continues until the goroutine stack exhausts available memory. Go does not optimize tail calls, so deliberate infinite recursion should be written as a loop.",
		GoodExample:     "if done { return }; visit(next)",
		BadExample:      "func visit() { visit() }",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (infiniteRecursionCheck) Run(pass *Pass) {
	for _, function := range pass.Functions {
		if function == nil || function.Blocks == nil {
			continue
		}
		exits := functionExitBlocks(function)
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok {
					continue
				}
				if _, spawnsGoroutine := instruction.(*ssa.Go); spawnsGoroutine || call.Common().StaticCallee() != function || !blockDominatesEveryExit(
					block,
					exits,
				) {
					continue
				}
				pass.ReportPos(call.Pos(), "recursive call has no path to a function exit")
			}
		}
	}
}

func functionExitBlocks(function *ssa.Function) []*ssa.BasicBlock {
	exits := make([]*ssa.BasicBlock, 0)
	for _, block := range function.Blocks {
		if len(block.Succs) == 0 {
			exits = append(exits, block)
		}
	}
	return exits
}

func blockDominatesEveryExit(block *ssa.BasicBlock, exits []*ssa.BasicBlock) bool {
	for _, exit := range exits {
		if !block.Dominates(exit) {
			return false
		}
	}
	return true
}
