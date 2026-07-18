package semantic

import (
	"fmt"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type sortNonSliceRule struct{}

func (sortNonSliceRule) Meta() Meta {
	return Meta{
		Code:            "sort-non-slice",
		Summary:         "detect sort.Slice calls with non-slice values",
		Explanation:     "sort.Slice, sort.SliceStable, and sort.SliceIsSorted accept any only for historical API reasons. Their first argument must hold a slice; passing another concrete type panics at runtime.",
		GoodExample:     "sort.Slice(values, less)",
		BadExample:      "sort.Slice(array, less)",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (sortNonSliceRule) Run(pass *Pass) {
	calls := pass.argumentsByCallPosition()
	for _, call := range pass.staticCallsInPackage("sort") {
		if !isSortSliceCall(call) || len(call.Common().Args) == 0 {
			continue
		}
		argument := unwrapSSAValue(call.Common().Args[0])
		if _, ok := types.Unalias(argument.Type()).Underlying().(*types.Slice); ok {
			continue
		}
		if _, unknown := types.Unalias(argument.Type()).Underlying().(*types.Interface); unknown && !isNilSSAConstant(argument) {
			continue
		}
		node := explicitCallArgument(calls[call.Pos()], 0, call.Pos())
		pass.Report(
			node,
			fmt.Sprintf("%s requires a slice, but the argument has type %s", sortSliceCallName(call), types.TypeString(argument.Type(), types.RelativeTo(pass.Types))),
		)
	}
}

func isSortSliceCall(call ssa.CallInstruction) bool {
	return sortSliceCallName(call) != ""
}

func sortSliceCallName(call ssa.CallInstruction) string {
	callee := call.Common().StaticCallee()
	if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil || callee.Object().Pkg().Path() != "sort" {
		return ""
	}
	switch callee.Object().Name() {
	case "Slice", "SliceStable", "SliceIsSorted":
		return "sort." + callee.Object().Name()
	default:
		return ""
	}
}

func isNilSSAConstant(value ssa.Value) bool {
	constant, ok := value.(*ssa.Const)
	return ok && constant.IsNil()
}
