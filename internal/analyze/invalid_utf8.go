package analyze

import (
	"go/constant"
	"unicode/utf8"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type invalidUTF8StringArgumentRule struct {}

func (invalidUTF8StringArgumentRule) Meta() Meta {
	return Meta{
		Code: "invalid-utf8",
		Summary: "detect invalid UTF-8 arguments to strings functions",
		Explanation: "The cutset and character-list arguments of selected strings functions must contain valid UTF-8.",
		GoodExample: `strings.Trim(value, "é")`,
		BadExample: `strings.Trim(value, "\xff")`,
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (invalidUTF8StringArgumentRule) Run(pass *Pass) {
	calls := pass.argumentsByCallPosition()
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok || !isUTF8StringsCall(call) || len(call.Common().Args) < 2 {
					continue
				}
				value := ssaConstant(call.Common().Args[1])
				if value == nil || value.Value == nil || value.Value.Kind() != constant.String {
					continue
				}
				if utf8.ValidString(constant.StringVal(value.Value)) {
					continue
				}
				node := explicitCallArgument(calls[call.Pos()], 1, call.Pos())
				pass.Report(node, "argument is not a valid UTF-8 encoded string")
			}
		}
	}
}

func isUTF8StringsCall(call ssa.CallInstruction) bool {
	callee := call.Common().StaticCallee()
	if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil || callee.Object().Pkg().Path() != "strings" {
		return false
	}
	switch callee.Object().Name() {
	case "IndexAny", "LastIndexAny", "ContainsAny", "Trim", "TrimLeft", "TrimRight":
		return true
	default:
		return false
	}
}
