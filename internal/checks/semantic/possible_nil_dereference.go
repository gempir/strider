package semantic

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type possibleNilDereferenceRule struct {}

type nilCheck struct {
	value ssa.Value
	nonNilPath *ssa.BasicBlock
}

func (possibleNilDereferenceRule) Meta() Meta {
	return Meta{
		Code: "possible-nil-dereference",
		Summary: "detect pointer dereferences not protected by their nil checks",
		Explanation: "Checking a pointer against nil is evidence that nil is a possible value. A dereference that is not dominated by the check's non-nil path may panic, commonly because it occurs before the check or because the nil branch reports an error but continues.",
		GoodExample: "if value == nil { return }; use(*value)",
		BadExample: "if value == nil { logError() }; use(*value)",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (possibleNilDereferenceRule) Run(pass *Pass) {
	for _, function := range pass.Functions {
		if function == nil || function.Blocks == nil {
			continue
		}
		checks := collectNilChecks(function)
		if len(checks) == 0 {
			continue
		}
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				if instruction.Pos() == token.NoPos {
					continue
				}
				pointer := dereferencedPointer(instruction)
				if pointer == nil || pointerCannotBeNil(pointer) {
					continue
				}
				pointer = normalizedNilValue(pointer)
				matched, protected := false, false
				for _, check := range checks {
					if normalizedNilValue(check.value) != pointer {
						continue
					}
					matched = true
					if check.nonNilPath != nil && check.nonNilPath.Dominates(block) {
						protected = true
						break
					}
				}
				if matched && !protected {
					pass.Report(positionNode{position: instruction.Pos()}, "pointer is dereferenced on a path where its nil check does not prove it is non-nil")
				}
			}
		}
	}
}

func collectNilChecks(function *ssa.Function) []nilCheck {
	checks := make([]nilCheck, 0, len(function.Blocks))
	for _, block := range function.Blocks {
		if len(block.Instrs) == 0 || len(block.Succs) < 2 {
			continue
		}
		branch, ok := block.Instrs[len(block.Instrs) - 1].(*ssa.If)
		if !ok {
			continue
		}
		comparison, ok := branch.Cond.(*ssa.BinOp)
		if !ok || (comparison.Op != token.EQL && comparison.Op != token.NEQ) {
			continue
		}
		var value ssa.Value
		switch {
		case isNilSSAConstant(comparison.X):
			value = comparison.Y
		case isNilSSAConstant(comparison.Y):
			value = comparison.X
		default:
			continue
		}
		nonNilSuccessor := 0
		if comparison.Op == token.EQL {
			nonNilSuccessor = 1
		}
		nonNilPath := block.Succs[nonNilSuccessor]
		if len(nonNilPath.Preds) != 1 {
			// The successor is also reachable from the nil branch, so reaching
			// it does not prove that the comparison selected the non-nil edge.
			nonNilPath = nil
		}
		checks = append(checks, nilCheck{value: value, nonNilPath: nonNilPath})
	}
	return checks
}

func dereferencedPointer(instruction ssa.Instruction) ssa.Value {
	switch instruction := instruction.(type) {
	case *ssa.UnOp:
		if instruction.Op == token.MUL {
			return instruction.X
		}
	case *ssa.Store:
		return instruction.Addr
	case *ssa.FieldAddr:
		return instruction.X
	case *ssa.IndexAddr:
		if _, slice := instruction.X.Type().Underlying().(*types.Slice); !slice {
			return instruction.X
		}
	}
	return nil
}

func normalizedNilValue(value ssa.Value) ssa.Value {
	for {
		switch current := value.(type) {
		case *ssa.ChangeType:
			value = current.X
		case *ssa.Convert:
			value = current.X
		case *ssa.ChangeInterface:
			value = current.X
		case *ssa.MakeInterface:
			value = current.X
		default:
			return value
		}
	}
}

func pointerCannotBeNil(value ssa.Value) bool {
	switch normalizedNilValue(value).(type) {
	case *ssa.Alloc, *ssa.FieldAddr, *ssa.IndexAddr, *ssa.Global:
		return true
	default:
		return false
	}
}
