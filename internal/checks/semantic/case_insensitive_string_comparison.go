package semantic

import (
	"go/ast"
	"go/token"

	"github.com/gempir/strider/internal/diagnostic"
)

type caseInsensitiveStringComparisonRule struct{}

func (caseInsensitiveStringComparisonRule) Meta() Meta {
	return Meta{
		Code:            "case-insensitive-string-comparison",
		Summary:         "detect allocating case conversions used only for comparison",
		Explanation:     "Converting both strings with strings.ToLower or strings.ToUpper allocates intermediate strings and processes each input fully. strings.EqualFold compares incrementally without those allocations and can stop at the first mismatch.",
		GoodExample:     "if strings.EqualFold(left, right) { use() }",
		BadExample:      "if strings.ToLower(left) == strings.ToLower(right) { use() }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (caseInsensitiveStringComparisonRule) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.BinaryExpr)(nil),
		},
		func(node ast.Node) bool {
			comparison,
				ok := node.(*ast.BinaryExpr)
			if !ok || (comparison.Op != token.EQL && comparison.Op != token.NEQ) {
				return true
			}
			left,
				ok := comparison.X.(*ast.CallExpr)
			if !ok || len(left.Args) != 1 {
				return true
			}
			right,
				ok := comparison.Y.(*ast.CallExpr)
			if !ok || len(right.Args) != 1 {
				return true
			}
			leftFunction := calledFunction(pass.TypesInfo, left.Fun)
			rightFunction := calledFunction(pass.TypesInfo, right.Fun)
			if leftFunction == nil || rightFunction != leftFunction || leftFunction.Pkg() == nil || leftFunction.Pkg().Path() != "strings" || (leftFunction.Name() != "ToLower" && leftFunction.Name() != "ToUpper") {
				return true
			}
			message := "use strings.EqualFold instead of converting both strings before comparison"
			if comparison.Op == token.NEQ {
				message = "use !strings.EqualFold instead of converting both strings before comparison"
			}
			pass.Report(comparison, message)
			return true
		},
	)
}

func (caseInsensitiveStringComparisonRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
