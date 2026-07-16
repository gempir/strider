package analyze

import (
	"go/ast"
	"go/constant"
	"strings"

	"github.com/gempir/strider/internal/diagnostic"
)

type unsafeFormattedURLHostPortRule struct{}

func (unsafeFormattedURLHostPortRule) Meta() Meta {
	return Meta{
		Code:            "unsafe-formatted-url-host-port",
		Summary:         "detect URL host and port construction that breaks IPv6",
		Explanation:     "Formatting a URL as `scheme://host:port` does not add the brackets required around IPv6 literals. Build the authority with net.JoinHostPort so IPv4, hostnames, and IPv6 addresses are all encoded correctly.",
		GoodExample:     `address := net.JoinHostPort(host, strconv.Itoa(port)); url := "http://" + address`,
		BadExample:      `url := fmt.Sprintf("http://%s:%d/path", host, port)`,
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (unsafeFormattedURLHostPortRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) < 3 {
				return true
			}
			function := calledFunction(pass.TypesInfo, call.Fun)
			if function == nil || function.Pkg() == nil || function.Pkg().Path() != "fmt" ||
				function.Name() != "Sprintf" {
				return true
			}
			value := pass.TypesInfo.Types[call.Args[0]].Value
			if value == nil || value.Kind() != constant.String ||
				!formatsURLHostAndPort(constant.StringVal(value)) {
				return true
			}
			pass.Report(
				call.Args[0],
				"formatted URL host and port may be invalid for IPv6; build the authority with net.JoinHostPort",
			)
			return true
		})
	}
}

func formatsURLHostAndPort(format string) bool {
	scheme := strings.Index(format, "://")
	if scheme < 0 {
		return false
	}
	authority := format[scheme+3:]
	end := len(authority)
	for _, separator := range []string{"/", "?", "#"} {
		if index := strings.Index(authority, separator); index >= 0 && index < end {
			end = index
		}
	}
	authority = authority[:end]
	return strings.Contains(authority, "%s:") || strings.Contains(authority, "%v:")
}
