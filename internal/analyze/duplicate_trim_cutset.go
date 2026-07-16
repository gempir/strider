package analyze

import (
	"fmt"
	"go/constant"

	"github.com/gempir/strider/internal/diagnostic"
	"golang.org/x/tools/go/ssa"
)

type duplicateTrimCutsetRule struct{}

func (duplicateTrimCutsetRule) Meta() Meta {
	return Meta{
		Code:            "duplicate-trim-cutset",
		Summary:         "detect duplicate characters in string trim cutsets",
		Explanation:     "strings.Trim, TrimLeft, and TrimRight interpret their second argument as a set of runes, not as a prefix or suffix. Duplicate runes have no effect and often reveal that a prefix-removal operation was intended.",
		GoodExample:     `strings.TrimPrefix(value, "prefix")`,
		BadExample:      `strings.TrimLeft(value, "letter")`,
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (duplicateTrimCutsetRule) Run(pass *Pass) {
	calls := argumentsByCallPosition(pass.Files)
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok || !isStringTrimCutsetCall(call) || len(call.Common().Args) <= 1 {
					continue
				}
				cutset := ssaConstant(call.Common().Args[1])
				if cutset == nil || cutset.Value == nil || cutset.Value.Kind() != constant.String {
					continue
				}
				duplicate, ok := duplicateRune(constant.StringVal(cutset.Value))
				if !ok {
					continue
				}
				node := explicitCallArgument(calls[call.Pos()], 1, call.Pos())
				pass.Report(
					node,
					fmt.Sprintf("trim cutset contains duplicate character %q", duplicate),
				)
			}
		}
	}
}

func isStringTrimCutsetCall(call ssa.CallInstruction) bool {
	return isStaticFunction(call, "strings", "Trim") ||
		isStaticFunction(call, "strings", "TrimLeft") ||
		isStaticFunction(call, "strings", "TrimRight")
}

func duplicateRune(value string) (rune, bool) {
	seen := make(map[rune]bool)
	for _, character := range value {
		if seen[character] {
			return character, true
		}
		seen[character] = true
	}
	return 0, false
}
