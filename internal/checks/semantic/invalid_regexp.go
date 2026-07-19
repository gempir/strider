package semantic

import (
	"go/constant"
	"go/token"
	"regexp"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type invalidRegexpRule struct{}

type positionNode struct {
	position token.Pos
}

func (invalidRegexpRule) Meta() Meta {
	return Meta{
		Code:            "invalid-regexp",
		Summary:         "detect invalid regular expressions",
		Explanation:     "Regular expressions passed as compile-time constants to regexp compilation and matching functions must be valid Go regular expressions.",
		GoodExample:     "regexp.MustCompile(`[a-z]+`)",
		BadExample:      "regexp.MustCompile(`[a-z`)",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (invalidRegexpRule) Run(pass *Pass) {
	calls := pass.firstArgumentsByCallPosition()
	for _, call := range pass.staticCallsInPackage("regexp") {
		if !isRegexpCompileCall(call) || len(call.Common().Args) == 0 {
			continue
		}
		value := ssaConstant(call.Common().Args[0])
		if value == nil || value.Value == nil || value.Value.Kind() != constant.String {
			continue
		}
		if _, err := regexp.Compile(constant.StringVal(value.Value)); err != nil {
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

func isRegexpCompileCall(call ssa.CallInstruction) bool {
	callee := call.Common().StaticCallee()
	if callee == nil {
		return false
	}
	function := callee.Object()
	if function == nil || function.Pkg() == nil || function.Pkg().Path() != "regexp" {
		return false
	}
	switch function.Name() {
	case "Compile", "MustCompile", "Match", "MatchReader", "MatchString":
		return true
	default:
		return false
	}
}

func ssaConstant(value ssa.Value) *ssa.Const {
	switch value := value.(type) {
	case *ssa.Const:
		return value
	case *ssa.MakeInterface:
		return ssaConstant(value.X)
	case *ssa.ChangeInterface:
		return ssaConstant(value.X)
	case *ssa.ChangeType:
		return ssaConstant(value.X)
	case *ssa.Convert:
		return ssaConstant(value.X)
	default:
		return nil
	}
}

func (node positionNode) Pos() token.Pos {
	return node.position
}

func (node positionNode) End() token.Pos {
	return node.position
}
