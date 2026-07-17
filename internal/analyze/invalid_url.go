package analyze

import (
	"fmt"
	"go/constant"
	"net/url"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type invalidURLRule struct {}

func (invalidURLRule) Meta() Meta {
	return Meta{
		Code: "invalid-url",
		Summary: "detect invalid URLs passed to net/url.Parse",
		Explanation: "Constant strings passed to net/url.Parse must satisfy Go's URL syntax.",
		GoodExample: `url.Parse("https://golang.org")`,
		BadExample: `url.Parse(":")`,
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (invalidURLRule) Run(pass *Pass) {
	calls := firstArgumentsByCallPosition(pass.Files)
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok || !isStaticFunction(call, "net/url", "Parse") || len(call.Common().Args) == 0 {
					continue
				}
				value := ssaConstant(call.Common().Args[0])
				if value == nil || value.Value == nil || value.Value.Kind() != constant.String {
					continue
				}
				rawURL := constant.StringVal(value.Value)
				if _, err := url.Parse(rawURL); err != nil {
					node := calls[call.Pos()]
					if node == nil {
						node = positionNode{position: call.Pos()}
					}
					pass.Report(node, fmt.Sprintf("%q is not a valid URL: %s", rawURL, err))
				}
			}
		}
	}
}
