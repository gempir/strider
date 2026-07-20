package semantic

import (
	"fmt"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type ineffectiveValueReceiverAssignmentRule struct{}

func (ineffectiveValueReceiverAssignmentRule) Meta() Meta {
	return Meta{
		Code:            "ineffective-value-receiver-assignment",
		Summary:         "detect field assignments that cannot escape a value receiver",
		Explanation:     "A method with a value receiver modifies only its local receiver copy. When an assigned field is never read afterward, the write has no observable effect and often indicates that the method should use a pointer receiver.",
		GoodExample:     "func (item *Item) Rename(name string) { item.Name = name }",
		BadExample:      "func (item Item) Rename(name string) { item.Name = name }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (ineffectiveValueReceiverAssignmentRule) Run(pass *Pass) {
	for _, function := range pass.Functions {
		receiver, fields, ok := valueReceiver(function)
		if !ok {
			continue
		}
		allocation, initialStore, ok := receiverAllocation(receiver)
		if !ok {
			continue
		}
		reads, writes, ok := receiverFieldAccesses(allocation, initialStore, fields.NumFields())
		if !ok {
			continue
		}
		for field, stores := range writes {
			for _, store := range stores {
				if anyReadReachableAfter(store, reads[field]) {
					continue
				}
				pass.ReportPos(
					store.Pos(),
					fmt.Sprintf(
						"assignment to value receiver field %s.%s has no observable effect",
						receiverTypeName(receiver.Type()),
						fields.Field(field).Name(),
					),
				)
			}
		}
	}
}

func valueReceiver(function *ssa.Function) (*ssa.Parameter, *types.Struct, bool) {
	if function == nil || function.Synthetic != "" || function.Signature == nil || function.Signature.Recv() == nil || len(function.Params) == 0 {
		return nil, nil, false
	}
	receiver := function.Params[0]
	fields, ok := types.Unalias(receiver.Type()).Underlying().(*types.Struct)
	return receiver, fields, ok
}

func receiverAllocation(receiver *ssa.Parameter) (*ssa.Alloc, *ssa.Store, bool) {
	references := receiver.Referrers()
	if references == nil {
		return nil, nil, false
	}
	var initialStore *ssa.Store
	for _, reference := range *references {
		switch reference := reference.(type) {
		case *ssa.Store:
			if initialStore != nil {
				return nil, nil, false
			}
			initialStore = reference
		case *ssa.DebugRef:
			continue
		default:
			return nil, nil, false
		}
	}
	if initialStore == nil {
		return nil, nil, false
	}
	allocation, ok := initialStore.Addr.(*ssa.Alloc)
	if !ok || allocation.Heap {
		return nil, nil, false
	}
	return allocation, initialStore, true
}

func receiverFieldAccesses(allocation *ssa.Alloc, initialStore *ssa.Store, fieldCount int) (map[int][]ssa.Instruction, map[int][]*ssa.Store, bool) {
	reads := make(map[int][]ssa.Instruction)
	writes := make(map[int][]*ssa.Store)
	references := allocation.Referrers()
	if references == nil {
		return reads, writes, true
	}
	for _, reference := range *references {
		switch reference := reference.(type) {
		case *ssa.FieldAddr:
			fieldReferences := reference.Referrers()
			if fieldReferences == nil {
				continue
			}
			for _, fieldReference := range *fieldReferences {
				switch fieldReference := fieldReference.(type) {
				case *ssa.Store:
					writes[reference.Field] = append(writes[reference.Field], fieldReference)
				case *ssa.UnOp:
					if fieldReference.Op == token.MUL {
						reads[reference.Field] = append(reads[reference.Field], fieldReference)
					}
				case *ssa.DebugRef:
					continue
				}
			}
		case *ssa.Store:
			if reference != initialStore {
				return nil, nil, false
			}
		case *ssa.UnOp:
			if reference.Op != token.MUL {
				return nil, nil, false
			}
			for field := range fieldCount {
				reads[field] = append(reads[field], reference)
			}
		case *ssa.DebugRef:
			continue
		default:
			return nil, nil, false
		}
	}
	return reads, writes, true
}

func anyReadReachableAfter(write *ssa.Store, reads []ssa.Instruction) bool {
	for _, read := range reads {
		if write.Block() == read.Block() {
			if instructionOffset(read) > instructionOffset(write) {
				return true
			}
			continue
		}
		if blockReachable(write.Block(), read.Block()) {
			return true
		}
	}
	return false
}

func instructionOffset(instruction ssa.Instruction) int {
	for index, candidate := range instruction.Block().Instrs {
		if candidate == instruction {
			return index
		}
	}
	return -1
}

func blockReachable(start, target *ssa.BasicBlock) bool {
	seen := make(map[*ssa.BasicBlock]bool)
	work := []*ssa.BasicBlock{
		start,
	}
	for len(work) != 0 {
		block := work[len(work)-1]
		work = work[:len(work)-1]
		if block == target {
			return true
		}
		if seen[block] {
			continue
		}
		seen[block] = true
		work = append(work, block.Succs...)
	}
	return false
}

func receiverTypeName(valueType types.Type) string {
	named, ok := types.Unalias(valueType).(*types.Named)
	if !ok {
		return valueType.String()
	}
	return named.Obj().Name()
}
