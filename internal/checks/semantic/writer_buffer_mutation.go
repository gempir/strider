package semantic

import (
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type writerBufferMutationRule struct{}

func (writerBufferMutationRule) Meta() Meta {
	return Meta{
		Code:            "writer-buffer-mutation",
		Summary:         "detect io.Writer implementations that modify their input buffer",
		Explanation:     "The io.Writer contract requires Write implementations not to modify the provided byte slice, even temporarily. Mutating an element or appending into the input can corrupt caller-owned data.",
		GoodExample:     "func (w *writer) Write(p []byte) (int, error) { return w.dst.Write(p) }",
		BadExample:      "func (w *writer) Write(p []byte) (int, error) { p[0] = 0; return len(p), nil }",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (writerBufferMutationRule) Run(pass *Pass) {
	for _, function := range pass.Functions {
		if !isWriterMethod(function) {
			continue
		}
		buffer := function.Params[1]
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				if modifiesWriterBuffer(instruction, buffer) {
					pass.Report(
						positionNode{position: instruction.Pos()},
						"io.Writer.Write must not modify the provided buffer, even temporarily",
					)
				}
			}
		}
	}
}

func isWriterMethod(function *ssa.Function) bool {
	signature := function.Signature
	if function.Name() != "Write" || signature.Recv() == nil || signature.Params().Len() != 1 || signature.Results().Len() != 2 || signature.Variadic() || len(
		function.Params,
	) < 2 {
		return false
	}
	slice, ok := signature.Params().At(0).Type().Underlying().(*types.Slice)
	if !ok || !types.Identical(slice.Elem(), types.Typ[types.Byte]) || !types.Identical(
		signature.Results().At(0).Type(),
		types.Typ[types.Int],
	) {
		return false
	}
	errorType := types.Universe.Lookup("error").Type()
	return types.Identical(signature.Results().At(1).Type(), errorType)
}

func modifiesWriterBuffer(instruction ssa.Instruction, buffer ssa.Value) bool {
	switch instruction := instruction.(type) {
	case *ssa.Store:
		address, ok := instruction.Addr.(*ssa.IndexAddr)
		return ok && address.X == buffer
	case *ssa.Call:
		builtin, ok := instruction.Common().Value.(*ssa.Builtin)
		return ok && builtin.Name() == "append" && len(instruction.Common().Args) != 0 && instruction.Common().Args[0] == buffer
	default:
		return false
	}
}
