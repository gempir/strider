package semantic

import (
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type swappedSeekArgumentsRule struct{}

func (swappedSeekArgumentsRule) Meta() Meta {
	return Meta{
		Code:            "swapped-seek-arguments",
		Summary:         "detect swapped io.Seeker.Seek arguments",
		Explanation:     "The first argument to Seek is an int64 byte offset and the second is an io.Seek* whence constant. Passing a whence constant first usually means the arguments were swapped.",
		GoodExample:     "seeker.Seek(0, io.SeekStart)",
		BadExample:      "seeker.Seek(io.SeekStart, 0)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (swappedSeekArgumentsRule) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
		},
		func(node ast.Node) bool {
			call,
				ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 || !isIOSeekConstant(pass, call.Args[0]) {
				return true
			}
			selector,
				ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Seek" || !hasSeekerSignature(pass, selector) {
				return true
			}
			pass.Report(call, "the first argument of io.Seeker is the offset, but an io.Seek* constant is being used instead")
			return true
		},
	)
}

func isIOSeekConstant(pass *Pass, expression ast.Expr) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	constant, ok := pass.TypesInfo.Uses[selector.Sel].(*types.Const)
	if !ok || constant.Pkg() == nil || constant.Pkg().Path() != "io" {
		return false
	}
	switch constant.Name() {
	case "SeekStart", "SeekCurrent", "SeekEnd":
		return true
	default:
		return false
	}
}

func hasSeekerSignature(pass *Pass, selector *ast.SelectorExpr) bool {
	selection := pass.TypesInfo.Selections[selector]
	if selection == nil || selection.Kind() != types.MethodVal {
		return false
	}
	signature, ok := selection.Type().(*types.Signature)
	if !ok || signature.Params().Len() != 2 || signature.Results().Len() != 2 || signature.Variadic() {
		return false
	}
	return types.Identical(signature.Params().At(0).Type(), types.Typ[types.Int64]) && types.Identical(signature.Params().At(1).Type(), types.Typ[types.Int]) && types.Identical(
		signature.Results().At(0).Type(),
		types.Typ[types.Int64],
	) && types.Identical(signature.Results().At(1).Type(), types.Universe.Lookup("error").Type())
}

func (swappedSeekArgumentsRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
