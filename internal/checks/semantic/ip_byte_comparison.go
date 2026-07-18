package semantic

import (
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type ipByteComparisonRule struct{}

func (ipByteComparisonRule) Meta() Meta {
	return Meta{
		Code:            "ip-byte-comparison",
		Summary:         "detect bytes.Equal comparisons between IP addresses",
		Explanation:     "An IPv4 address stored in net.IP may use either a 4-byte or 16-byte representation. bytes.Equal treats those representations as different; net.IP.Equal compares their address values correctly.",
		GoodExample:     "left.Equal(right)",
		BadExample:      "bytes.Equal(left, right)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (ipByteComparisonRule) Run(pass *Pass) {
	calls := pass.argumentsByCallPosition()
	for _, call := range pass.staticCallsInPackage("bytes") {
		if !isStaticFunction(call, "bytes", "Equal") || len(call.Common().Args) != 2 || !convertedFromNamedType(
			call.Common().Args[0],
			"net",
			"IP",
		) || !convertedFromNamedType(call.Common().Args[1], "net", "IP") {
			continue
		}
		node := explicitCallArgument(calls[call.Pos()], 0, call.Pos())
		pass.Report(node, "use net.IP.Equal to compare IP addresses, not bytes.Equal")
	}
}

func convertedFromNamedType(value ssa.Value, packagePath, name string) bool {
	change, ok := value.(*ssa.ChangeType)
	if !ok {
		return false
	}
	named, ok := types.Unalias(change.X.Type()).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == packagePath && named.Obj().Name() == name
}
