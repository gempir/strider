package semantic

import (
	"go/constant"
	"go/token"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type zeroReplacementLimitRule struct{}

func (zeroReplacementLimitRule) Meta() Meta {
	return Meta{
		Code:            "zero-replacement-limit",
		Summary:         "detect replacement calls with a zero limit",
		Explanation:     "The final argument to strings.Replace and bytes.Replace is the maximum number of replacements. Zero replaces nothing; use a negative value or ReplaceAll to replace every occurrence.",
		GoodExample:     "strings.ReplaceAll(value, old, replacement)",
		BadExample:      "strings.Replace(value, old, replacement, 0)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (zeroReplacementLimitRule) Run(pass *Pass) {
	calls := pass.argumentsByCallPosition()
	for _, packagePath := range []string{
		"bytes",
		"strings",
	} {
		for _, call := range pass.staticCallsInPackage(packagePath) {
			if len(call.Common().Args) <= 3 || !isReplacementCall(call) {
				continue
			}
			limit := ssaConstant(call.Common().Args[3])
			if limit == nil || limit.Value == nil || limit.Value.Kind() != constant.Int || !constant.Compare(limit.Value, token.EQL, constant.MakeInt64(0)) {
				continue
			}
			node := explicitCallArgument(calls[call.Pos()], 3, call.Pos())
			pass.Report(node, "a replacement limit of zero replaces nothing; use -1 or ReplaceAll to replace every occurrence")
		}
	}
}

func isReplacementCall(call ssa.CallInstruction) bool {
	return isStaticFunction(call, "strings", "Replace") || isStaticFunction(call, "bytes", "Replace")
}
