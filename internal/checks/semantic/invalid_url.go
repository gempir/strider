package semantic

import (
	"fmt"
	"go/constant"
	"net/url"

	"github.com/gempir/strider/internal/diagnostic"
)

type invalidURLRule struct{}

func (invalidURLRule) Meta() Meta {
	return Meta{
		Code:            "invalid-url",
		Summary:         "detect invalid URLs passed to net/url.Parse",
		Explanation:     "Constant strings passed to net/url.Parse must satisfy Go's URL syntax.",
		GoodExample:     `url.Parse("https://golang.org")`,
		BadExample:      `url.Parse(":")`,
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (invalidURLRule) Run(pass *Pass) {
	calls := pass.firstArgumentsByCallPosition()
	for _, call := range pass.staticCallsInPackage("net/url") {
		if !isStaticFunction(call, "net/url", "Parse") || len(call.Common().Args) == 0 {
			continue
		}
		value := ssaConstant(call.Common().Args[0])
		if value == nil || value.Value == nil || value.Value.Kind() != constant.String {
			continue
		}
		rawURL := constant.StringVal(value.Value)
		if _, err := url.Parse(rawURL); err != nil {
			message := fmt.Sprintf("%q is not a valid URL: %s", rawURL, err)
			if node := calls[call.Pos()]; node != nil {
				pass.Report(node, message)
			} else {
				pass.ReportPos(call.Pos(), message)
			}
		}
	}
}
