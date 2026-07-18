package semantic

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type timerResetDrainRaceRule struct{}

func (timerResetDrainRaceRule) Meta() Meta {
	return Meta{
		Code:            "timer-reset-drain-race",
		Summary:         "detect attempts to drain a timer based on Reset's result",
		Explanation:     "Using time.Timer.Reset's boolean result to decide whether to receive from a timer channel is racy on older timer implementations and can block with current synchronous timer channels. Stop and drain before resetting when compatibility requires it, or reset without conditionally draining afterward.",
		GoodExample:     "if !timer.Stop() { select { case <-timer.C: default: } }\ntimer.Reset(delay)",
		BadExample:      "if !timer.Reset(delay) { <-timer.C }",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (timerResetDrainRaceRule) Run(pass *Pass) {
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(*ssa.Call)
				if !ok || !isTimerResetCall(call) {
					continue
				}
				for _, conditional := range conditionalUses(call) {
					if conditionBranchesReceiveTime(conditional) {
						pass.Report(positionNode{position: call.Pos()}, "do not use Timer.Reset's return value to decide whether to drain the timer channel")
						break
					}
				}
			}
		}
	}
}

func isTimerResetCall(call *ssa.Call) bool {
	callee := call.Common().StaticCallee()
	if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil || callee.Object().Pkg().Path() != "time" || callee.Object().Name() != "Reset" {
		return false
	}
	function, ok := callee.Object().(*types.Func)
	if !ok {
		return false
	}
	signature, _ := function.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		return false
	}
	pointer, ok := types.Unalias(signature.Recv().Type()).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(pointer.Elem()).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "time" && named.Obj().Name() == "Timer"
}

func conditionalUses(value ssa.Value) []*ssa.If {
	conditionals := make([]*ssa.If, 0)
	seen := make(map[ssa.Value]bool)
	var visit func(ssa.Value)
	visit = func(current ssa.Value) {
		if current == nil || seen[current] {
			return
		}
		seen[current] = true
		references := current.Referrers()
		if references == nil {
			return
		}
		for _, reference := range *references {
			switch reference := reference.(type) {
			case *ssa.If:
				conditionals = append(conditionals, reference)
			case *ssa.UnOp:
				if reference.Op == token.NOT {
					visit(reference)
				}
			}
		}
	}
	visit(value)
	return conditionals
}

func conditionBranchesReceiveTime(conditional *ssa.If) bool {
	for _, successor := range conditional.Block().Succs {
		if len(successor.Preds) != 1 {
			continue
		}
		seen := make(map[*ssa.BasicBlock]bool)
		work := []*ssa.BasicBlock{successor}
		for len(work) != 0 {
			block := work[len(work)-1]
			work = work[:len(work)-1]
			if seen[block] || !successor.Dominates(block) {
				continue
			}
			seen[block] = true
			for _, instruction := range block.Instrs {
				receive, ok := instruction.(*ssa.UnOp)
				if ok && receive.Op == token.ARROW && isTimeChannel(receive.X.Type()) {
					return true
				}
			}
			work = append(work, block.Succs...)
		}
	}
	return false
}

func isTimeChannel(valueType types.Type) bool {
	channel, ok := types.Unalias(valueType).Underlying().(*types.Chan)
	if !ok {
		return false
	}
	named, ok := types.Unalias(channel.Elem()).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "time" && named.Obj().Name() == "Time"
}
