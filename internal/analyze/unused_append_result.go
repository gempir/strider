package analyze

import (
	"github.com/gempir/strider/internal/diagnostic"
	"golang.org/x/tools/go/ssa"
)

type unusedAppendResultRule struct{}

func (unusedAppendResultRule) Meta() Meta {
	return Meta{
		Code:            "unused-append-result",
		Summary:         "detect append results that can never be observed",
		Explanation:     "append returns the updated slice header. Discarding that result loses any new length or reallocated backing array. The analyzer reports only function-local slices whose backing storage has not escaped or been observably aliased.",
		GoodExample:     "values = append(values, item)",
		BadExample:      "values := make([]int, 0); values = append(values, item) // values is never read again",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (unusedAppendResultRule) Run(pass *Pass) {
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := appendCall(instruction)
				if !ok || call.Referrers() == nil || appendResultUsed(call) ||
					len(call.Common().Args) == 0 {
					continue
				}
				origins := make(map[ssa.Value]bool)
				if !validAppendOrigin(call.Common().Args[0], origins) ||
					appendOriginEscapes(call, origins) {
					continue
				}
				pass.Report(
					positionNode{position: call.Pos()},
					"result of append is never used or observed",
				)
			}
		}
	}
}

func appendCall(value any) (*ssa.Call, bool) {
	call, ok := value.(*ssa.Call)
	if !ok {
		return nil, false
	}
	builtin, ok := call.Common().Value.(*ssa.Builtin)
	return call, ok && builtin.Name() == "append"
}

func appendResultUsed(call *ssa.Call) bool {
	visited := make(map[ssa.Instruction]bool)
	var used bool
	var walk func([]ssa.Instruction)
	walk = func(references []ssa.Instruction) {
		for _, reference := range references {
			if used || visited[reference] {
				continue
			}
			visited[reference] = true
			switch reference := reference.(type) {
			case *ssa.DebugRef:
				continue
			case *ssa.Phi:
				if reference.Referrers() != nil {
					walk(*reference.Referrers())
				}
			case ssa.Value:
				if chained, ok := appendCall(reference); ok {
					if chained.Referrers() != nil {
						walk(*chained.Referrers())
					}
					continue
				}
				used = true
			default:
				used = true
			}
		}
	}
	walk(*call.Referrers())
	return used
}

func validAppendOrigin(value ssa.Value, seen map[ssa.Value]bool) bool {
	if seen[value] {
		return true
	}
	seen[value] = true
	switch value := value.(type) {
	case *ssa.Phi:
		for _, edge := range value.Edges {
			if !validAppendOrigin(edge, seen) {
				return false
			}
		}
		return true
	case *ssa.Slice:
		return validAppendOrigin(value.X, seen)
	case *ssa.Const, *ssa.MakeSlice, *ssa.Alloc:
		return true
	case *ssa.Call:
		call, ok := appendCall(value)
		return ok && len(call.Common().Args) != 0 &&
			validAppendOrigin(call.Common().Args[0], seen)
	default:
		return false
	}
}

func appendOriginEscapes(target *ssa.Call, origins map[ssa.Value]bool) bool {
	allowed := make(map[ssa.Instruction]bool)
	for value := range origins {
		if instruction, ok := value.(ssa.Instruction); ok {
			allowed[instruction] = true
		}
	}
	allowed[target] = true
	visited := make(map[ssa.Instruction]bool)
	for origin := range origins {
		if appendValueEscapes(origin, allowed, visited) {
			return true
		}
	}
	return false
}

func appendValueEscapes(
	value ssa.Value,
	allowed map[ssa.Instruction]bool,
	visited map[ssa.Instruction]bool,
) bool {
	references := value.Referrers()
	if references == nil {
		return false
	}
	for _, reference := range *references {
		if allowed[reference] || visited[reference] {
			continue
		}
		visited[reference] = true
		switch reference := reference.(type) {
		case *ssa.DebugRef:
			continue
		case *ssa.Phi:
			if appendValueEscapes(reference, allowed, visited) {
				return true
			}
		case *ssa.Slice:
			if appendValueEscapes(reference, allowed, visited) {
				return true
			}
		case *ssa.MakeSlice, *ssa.Alloc:
			value, ok := reference.(ssa.Value)
			if ok && appendValueEscapes(value, allowed, visited) {
				return true
			}
		default:
			return true
		}
	}
	return false
}
