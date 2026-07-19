package semantic

import (
	"go/constant"
	"strings"
	"time"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type invalidTimeParseRule struct{}

func (invalidTimeParseRule) Meta() Meta {
	return Meta{
		Code:            "invalid-time-layout",
		Summary:         "detect invalid time.Parse layouts",
		Explanation:     "time.Parse layouts must represent Go's reference time Mon Jan 2 15:04:05 MST 2006 rather than using conventional date-format placeholders.",
		GoodExample:     "time.Parse(\"2006-01-02\", value)",
		BadExample:      "time.Parse(\"YYYY-MM-DD\", value)",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (invalidTimeParseRule) Run(pass *Pass) {
	calls := pass.firstArgumentsByCallPosition()
	for _, call := range pass.staticCallsInPackage("time") {
		if !isStaticFunction(call, "time", "Parse") || len(call.Common().Args) == 0 {
			continue
		}
		value := ssaConstant(call.Common().Args[0])
		if value == nil || value.Value == nil || value.Value.Kind() != constant.String {
			continue
		}
		layout := constant.StringVal(value.Value)
		layout = strings.ReplaceAll(layout, "_", " ")
		layout = strings.ReplaceAll(layout, "Z", "-")
		if _, err := time.Parse(layout, layout); err != nil {
			node := calls[call.Pos()]
			if node == nil {
				node = positionNode{
					position: call.Pos(),
				}
			}
			pass.Report(node, err.Error())
		}
	}
}

func isStaticFunction(call ssa.CallInstruction, packagePath, name string) bool {
	callee := call.Common().StaticCallee()
	if callee == nil {
		return false
	}
	function := callee.Object()
	return function != nil && function.Pkg() != nil && function.Pkg().Path() == packagePath && function.Name() == name
}
