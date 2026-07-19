package semantic

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
		Summary:         "detect URL and network-address construction that breaks IPv6",
		Explanation:     "Formatting a URL or a network address as `host:port` does not add the brackets required around IPv6 literals. Build the authority with net.JoinHostPort so IPv4, hostnames, and IPv6 addresses are all encoded correctly.",
		GoodExample:     `address := net.JoinHostPort(host, strconv.Itoa(port)); url := "http://" + address`,
		BadExample:      `url := fmt.Sprintf("http://%s:%d/path", host, port)`,
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (unsafeFormattedURLHostPortRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				call,
					ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				formatted := formattedHostPortCall(pass, call)
				if formatted == nil {
					argument := networkAddressArgument(pass, call)
					formatted = bareFormattedHostPortCall(pass, argument)
				}
				if formatted == nil {
					return true
				}
				pass.Report(formatted.Args[0], "formatted host and port may be invalid for IPv6; build the address with net.JoinHostPort")
				return true
			},
		)
	}
}

func formattedHostPortCall(pass *Pass, call *ast.CallExpr) *ast.CallExpr {
	if call == nil || len(call.Args) < 3 || !isPackageFunction(pass.TypesInfo, call.Fun, "fmt", "Sprintf") {
		return nil
	}
	value := pass.TypesInfo.Types[call.Args[0]].Value
	if value == nil || value.Kind() != constant.String || !formatsURLHostAndPort(constant.StringVal(value)) {
		return nil
	}
	return call
}

func bareFormattedHostPortCall(pass *Pass, expression ast.Expr) *ast.CallExpr {
	call, _ := ast.Unparen(expression).(*ast.CallExpr)
	if call == nil || len(call.Args) < 3 || !isPackageFunction(pass.TypesInfo, call.Fun, "fmt", "Sprintf") {
		return nil
	}
	value := pass.TypesInfo.Types[call.Args[0]].Value
	if value == nil || value.Kind() != constant.String {
		return nil
	}
	format := constant.StringVal(value)
	if format != "%s:%d" && format != "%s:%s" && format != "%v:%d" && format != "%v:%s" {
		return nil
	}
	return call
}

func networkAddressArgument(pass *Pass, call *ast.CallExpr) ast.Expr {
	function := calledFunction(pass.TypesInfo, call.Fun)
	if function == nil || function.Pkg() == nil {
		return nil
	}
	index := -1
	switch function.Pkg().Path() {
	case "net":
		switch function.Name() {
		case "Dial", "DialTimeout", "Listen", "ListenPacket", "ResolveIPAddr", "ResolveTCPAddr", "ResolveUDPAddr", "ResolveUnixAddr":
			index = 1
		}
	case "net/http":
		switch function.Name() {
		case "ListenAndServe", "ListenAndServeTLS":
			index = 0
		}
	case "crypto/tls":
		if function.Name() == "Dial" {
			index = 1
		} else if function.Name() == "DialWithDialer" {
			index = 2
		}
	}
	if index < 0 || index >= len(call.Args) {
		return nil
	}
	return call.Args[index]
}

func formatsURLHostAndPort(format string) bool {
	scheme := strings.Index(format, "://")
	if scheme < 0 {
		return false
	}
	authority := format[scheme+3:]
	end := len(authority)
	for _, separator := range []string{
		"/",
		"?",
		"#",
	} {
		if index := strings.Index(authority, separator); index >= 0 && index < end {
			end = index
		}
	}
	authority = authority[:end]
	return strings.Contains(authority, "%s:") || strings.Contains(authority, "%v:")
}
