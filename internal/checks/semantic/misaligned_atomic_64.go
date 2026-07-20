package semantic

import (
	"fmt"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type misalignedAtomic64Rule struct{}

func (misalignedAtomic64Rule) Meta() Meta {
	return Meta{
		Code:            "misaligned-atomic-64",
		Summary:         "detect misaligned 64-bit atomic field access on 32-bit targets",
		Explanation:     "On 32-bit ARM, x86, and MIPS targets, callers must ensure 64-bit words passed to legacy sync/atomic functions are aligned to 8 bytes. A uint64 field after a narrow field may not satisfy that requirement.",
		GoodExample:     "type counters struct { total uint64; ready uint32 }",
		BadExample:      "type counters struct { ready uint32; total uint64 }",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (misalignedAtomic64Rule) Run(pass *Pass) {
	if pass.TypesSizes == nil || pass.TypesSizes.Sizeof(types.Typ[types.Uintptr]) != 4 {
		return
	}
	calls := pass.argumentsByCallPosition()
	for _, call := range pass.staticCallsInPackage("sync/atomic") {
		if !isAtomic64Call(call) || len(call.Common().Args) == 0 {
			continue
		}
		field, structure, ok := atomicFieldAddress(call.Common().Args[0])
		if !ok || field >= structure.NumFields() {
			continue
		}
		fields := make([]*types.Var, field+1)
		for index := range fields {
			fields[index] = structure.Field(index)
		}
		offset := pass.TypesSizes.Offsetsof(fields)[field]
		if offset%8 == 0 {
			continue
		}
		node := explicitCallArgument(calls[call.Pos()], 0, call.Pos())
		pass.Report(node, fmt.Sprintf("field %s is not 64-bit aligned for atomic access on this target", structure.Field(field).Name()))
	}
}

func isAtomic64Call(call ssa.CallInstruction) bool {
	callee := call.Common().StaticCallee()
	if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil || callee.Object().Pkg().Path() != "sync/atomic" {
		return false
	}
	switch callee.Object().Name() {
	case "AddInt64", "AddUint64", "CompareAndSwapInt64", "CompareAndSwapUint64", "LoadInt64", "LoadUint64", "StoreInt64", "StoreUint64", "SwapInt64", "SwapUint64":
		return true
	default:
		return false
	}
}

func atomicFieldAddress(value ssa.Value) (int, *types.Struct, bool) {
	for {
		switch current := value.(type) {
		case *ssa.ChangeType:
			value = current.X
		case *ssa.Convert:
			value = current.X
		case *ssa.FieldAddr:
			pointer, ok := types.Unalias(current.X.Type()).Underlying().(*types.Pointer)
			if !ok {
				return 0, nil, false
			}
			structure, ok := types.Unalias(pointer.Elem()).Underlying().(*types.Struct)
			return current.Field, structure, ok
		default:
			return 0, nil, false
		}
	}
}

func (misalignedAtomic64Rule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"sync/atomic",
		},
	}
}
