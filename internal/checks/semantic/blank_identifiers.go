package semantic

import (
	"go/ast"

	"github.com/gempir/strider/internal/diagnostic"
)

type excessiveBlankIdentifiersCheck struct{}

func (excessiveBlankIdentifiersCheck) Meta() Meta {
	return Meta{
		Code:            "excessive-blank-identifiers",
		Summary:         "detect assignments that discard too many results",
		Explanation:     "Discarding several adjacent results hides the contract of the called function and makes it easy to overlook an important value. Name the results that matter or return a cohesive result type.",
		GoodExample:     "value, metadata, err := load(); _ = metadata",
		BadExample:      "value, _, _, _, err := load()",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (excessiveBlankIdentifiersCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.AssignStmt)(nil),
		},
		func(node ast.Node) bool {
			assignment, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			blanks := 0
			for _, expression := range assignment.Lhs {
				identifier, _ := expression.(*ast.Ident)
				if identifier != nil && identifier.Name == "_" {
					blanks++
				}
			}
			if blanks >= 3 {
				pass.Report(assignment, "assignment discards three or more results; name meaningful results or simplify the return contract")
			}
			return true
		},
	)
}

func (excessiveBlankIdentifiersCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
