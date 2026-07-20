package semantic

import (
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type nonPointerSyncPoolValueRule struct{}

func (nonPointerSyncPoolValueRule) Meta() Meta {
	return Meta{
		Code:            "non-pointer-sync-pool-value",
		Summary:         "detect sync.Pool values that allocate while being stored",
		Explanation:     "sync.Pool.Put accepts an interface. Storing a concrete non-pointer value requires boxing it on the heap, adding the allocation the pool is intended to avoid. Slices are also boxed because the slice header itself is a value; store a pointer to the reusable value instead.",
		GoodExample:     "buffer := make([]byte, 0, 4096); pool.Put(&buffer)",
		BadExample:      "buffer := make([]byte, 0, 4096); pool.Put(buffer)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (nonPointerSyncPoolValueRule) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
		},
		func(node ast.Node) bool {
			call,
				ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 || !syncPoolPutCall(pass, call) {
				return true
			}
			valueType := pass.TypesInfo.TypeOf(call.Args[0])
			if poolPointerLike(valueType) {
				return true
			}
			pass.Report(call.Args[0], "store a pointer-like value in sync.Pool to avoid an allocation during Put")
			return true
		},
	)
}

func syncPoolPutCall(pass *Pass, call *ast.CallExpr) bool {
	function := calledFunction(pass.TypesInfo, call.Fun)
	if function == nil || function.Pkg() == nil || function.Pkg().Path() != "sync" || function.Name() != "Put" {
		return false
	}
	signature, _ := function.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		return false
	}
	pointer, ok := signature.Recv().Type().Underlying().(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := pointer.Elem().(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "sync" && named.Obj().Name() == "Pool"
}

func poolPointerLike(valueType types.Type) bool {
	if valueType == nil {
		return true
	}
	switch underlying := valueType.Underlying().(type) {
	case *types.Pointer, *types.Chan, *types.Map, *types.Signature, *types.Interface:
		return true
	case *types.Basic:
		return underlying.Kind() == types.UnsafePointer
	default:
		return false
	}
}

func (nonPointerSyncPoolValueRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
