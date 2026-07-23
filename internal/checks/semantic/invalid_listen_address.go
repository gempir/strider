//strider:ignore-file cognitive-complexity,cyclomatic-complexity
package semantic

import (
	"fmt"
	"go/constant"
	"net"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type invalidListenAddressCheck struct{}

func (invalidListenAddressCheck) Meta() Meta {
	return Meta{
		Code:            "invalid-listen-address",
		Summary:         "detect invalid constant HTTP listen addresses",
		Explanation:     "HTTP server listen functions expect a host:port pair. The host or port may be omitted, but the separator, numeric port range, and service-name syntax must still be valid.",
		GoodExample:     `http.ListenAndServe(":8080", handler)`,
		BadExample:      `http.ListenAndServe("localhost", handler)`,
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (invalidListenAddressCheck) Run(pass *Pass) {
	calls := pass.argumentsByCallPosition()
	for _, call := range pass.staticCallsInPackage("net/http") {
		if !isHTTPListenCall(call) || len(call.Common().Args) == 0 {
			continue
		}
		address := ssaConstant(call.Common().Args[0])
		if address == nil || address.Value == nil || address.Value.Kind() != constant.String {
			continue
		}
		value := constant.StringVal(address.Value)
		if validListenAddress(value) {
			continue
		}
		node := explicitCallArgument(calls[call.Pos()], 0, call.Pos())
		pass.Report(node, fmt.Sprintf("%q is not a valid host:port listen address", value))
	}
}

func isHTTPListenCall(call ssa.CallInstruction) bool {
	return isStaticFunction(call, "net/http", "ListenAndServe") || isStaticFunction(call, "net/http", "ListenAndServeTLS")
}

func validListenAddress(address string) bool {
	if address == "" {
		return true
	}
	_, port, err := net.SplitHostPort(address)
	return err == nil && validListenPort(port)
}

func validListenPort(port string) bool {
	if port == "" {
		return true
	}
	number, err := strconv.ParseUint(port, 10, 16)
	if err == nil {
		return number <= 65535
	}
	if len(port) < 1 || len(port) > 15 || port[0] == '-' || port[len(port)-1] == '-' || strings.Contains(port, "--") {
		return false
	}
	hasLetter := false
	for _, character := range port {
		switch {
		case character >= 'A' && character <= 'Z', character >= 'a' && character <= 'z':
			hasLetter = true
		case character >= '0' && character <= '9':
		case character == '-':
		default:
			return false
		}
	}
	return hasLetter
}
