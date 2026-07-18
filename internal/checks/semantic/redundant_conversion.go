package semantic

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type redundantConversionRule struct{}

func (redundantConversionRule) Meta() Meta {
	return Meta{
		Code:            "redundant-conversion",
		Summary:         "detect conversions to the value's existing type",
		Explanation:     "A conversion whose source and target are exactly the same Go type cannot change the value or its method set. Removing it makes the intended type flow clearer without changing behavior.",
		GoodExample:     "normalized := UserID(rawID)",
		BadExample:      "normalized := UserID(existingUserID)",
		DefaultSeverity: diagnostic.SeverityNote,
	}
}

func (redundantConversionRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				call,
					ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) != 1 || !pass.TypesInfo.Types[call.Fun].IsType() {
					return true
				}
				source := pass.TypesInfo.TypeOf(call.Args[0])
				target := pass.TypesInfo.TypeOf(call)
				if source == nil || target == nil || !types.Identical(source, target) {
					return true
				}
				pass.Report(
					call,
					fmt.Sprintf("conversion from %s to the identical type is redundant", types.TypeString(target, performanceTypeQualifier(pass.Types))),
				)
				return true
			},
		)
	}
}
