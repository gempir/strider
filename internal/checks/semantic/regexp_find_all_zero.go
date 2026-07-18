package semantic

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type regexpFindAllZeroRule struct{}

func (regexpFindAllZeroRule) Meta() Meta {
	return Meta{
		Code:            "regexp-find-all-zero",
		Summary:         "detect regexp FindAll calls with n equal to zero",
		Explanation:     "Regexp FindAll methods return at most n matches when n is non-negative. Passing zero always returns no results; use a negative value to request all matches.",
		GoodExample:     `matches := expression.FindAllString(input, -1)`,
		BadExample:      `matches := expression.FindAllString(input, 0)`,
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (regexpFindAllZeroRule) Run(pass *Pass) {
	calls := pass.argumentsByCallPosition()
	for _, call := range pass.staticCallsInPackage("regexp") {
		if !isRegexpFindAllCall(call) || len(call.Common().Args) < 3 {
			continue
		}
		value, ok := call.Common().Args[2].(*ssa.Const)
		if !ok || value.Value == nil || value.Value.Kind() != constant.Int {
			continue
		}
		n, exact := constant.Int64Val(value.Value)
		if !exact || n != 0 {
			continue
		}
		node := explicitCallArgument(calls[call.Pos()], 1, call.Pos())
		pass.Report(
			node,
			"calling a FindAll method with n == 0 will return no results, did you mean -1?",
		)
	}
}

func isRegexpFindAllCall(call ssa.CallInstruction) bool {
	callee := call.Common().StaticCallee()
	if callee == nil {
		return false
	}
	function, ok := callee.Object().(*types.Func)
	if !ok || function.Pkg() == nil || function.Pkg().Path() != "regexp" || function.Type().(*types.Signature).Recv() == nil {
		return false
	}
	switch function.Name() {
	case "FindAll", "FindAllIndex", "FindAllString", "FindAllStringIndex", "FindAllStringSubmatch", "FindAllStringSubmatchIndex", "FindAllSubmatch", "FindAllSubmatchIndex":
		return true
	default:
		return false
	}
}

func explicitCallArgument(arguments []ast.Node, index int, position token.Pos) ast.Node {
	if index >= 0 && index < len(arguments) {
		return arguments[index]
	}
	if len(arguments) != 0 {
		return arguments[0]
	}
	return positionNode{position: position}
}
