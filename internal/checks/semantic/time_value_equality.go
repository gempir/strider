package semantic

import (
	"go/ast"
	"go/token"

	"github.com/gempir/strider/internal/diagnostic"
)

type timeValueEqualityCheck struct{}

func (timeValueEqualityCheck) Meta() Meta {
	return Meta{
		Code:            "time-value-equality",
		Summary:         "compare time.Time values with Time.Equal",
		Explanation:     "The == and != operators compare every field of time.Time, including monotonic-clock and location representation details. Time.Equal compares the represented instants and is the intended equality operation.",
		GoodExample:     "if left.Equal(right) { use(left) }",
		BadExample:      "if left == right { use(left) }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (timeValueEqualityCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.BinaryExpr)(nil),
		},
		func(node ast.Node) bool {
			binary,
				ok := node.(*ast.BinaryExpr)
			if !ok || (binary.Op != token.EQL && binary.Op != token.NEQ) {
				return true
			}
			if !isNamedType(pass.TypesInfo.TypeOf(binary.X), "time", "Time") || !isNamedType(pass.TypesInfo.TypeOf(binary.Y), "time", "Time") {
				return true
			}
			pass.Report(binary, "compare time.Time values with Time.Equal instead of == or !=")
			return true
		},
	)
}

func (timeValueEqualityCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
