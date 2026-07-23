package semantic

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"

	"github.com/gempir/strider/internal/diagnostic"
)

type suspiciousSleepCheck struct{}

func (suspiciousSleepCheck) Meta() Meta {
	return Meta{
		Code:            "suspicious-sleep",
		Summary:         "detect suspiciously small time.Sleep constants",
		Explanation:     "Bare integer literals passed to time.Sleep are nanoseconds. Values from 1 through 120 usually indicate a missing time unit; multiply by time.Nanosecond when the small duration is intentional.",
		GoodExample:     "time.Sleep(5 * time.Second)",
		BadExample:      "time.Sleep(5)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (suspiciousSleepCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
		},
		func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 || !isPackageFunction(pass.TypesInfo, call.Fun, "time", "Sleep") {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.INT {
				return true
			}
			value := pass.TypesInfo.Types[literal].Value
			nanoseconds, exact := constant.Int64Val(value)
			if !exact || nanoseconds == 0 || nanoseconds > 120 {
				return true
			}
			pass.Report(literal, fmt.Sprintf("sleeping for %d nanoseconds is probably a bug; be explicit if it isn't", nanoseconds))
			return true
		},
	)
}
