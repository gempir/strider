package analyze

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type unreachableTypeSwitchCaseRule struct{}

func (unreachableTypeSwitchCaseRule) Meta() Meta {
	return Meta{
		Code:            "unreachable-type-switch-case",
		Summary:         "detect type-switch cases hidden by earlier interfaces",
		Explanation:     "Type-switch cases are evaluated in source order. A later concrete type or narrower interface is unreachable when it necessarily implements an interface listed by an earlier case.",
		GoodExample:     "switch value.(type) { case io.ReadCloser: use(); case io.Reader: use() }",
		BadExample:      "switch value.(type) { case io.Reader: use(); case io.ReadCloser: use() }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (unreachableTypeSwitchCaseRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switchStatement, ok := node.(*ast.TypeSwitchStmt)
			if !ok {
				return true
			}
			cases := typeSwitchCases(pass, switchStatement)
			for earlierIndex, earlier := range cases {
				for _, later := range cases[earlierIndex+1:] {
					first, hidden, ok := subsumingCaseTypes(earlier.types, later.types)
					if !ok {
						continue
					}
					pass.Report(
						later.clause,
						fmt.Sprintf(
							"unreachable type-switch case: %s always matches before %s",
							conciseAnalysisType(pass, first),
							conciseAnalysisType(pass, hidden),
						),
					)
				}
			}
			return true
		})
	}
}

func conciseAnalysisType(pass *Pass, valueType types.Type) string {
	return types.TypeString(valueType, func(pkg *types.Package) string {
		if pkg == pass.Types {
			return ""
		}
		return pkg.Name()
	})
}

type typedCaseClause struct {
	clause *ast.CaseClause
	types  []types.Type
}

func typeSwitchCases(pass *Pass, statement *ast.TypeSwitchStmt) []typedCaseClause {
	cases := make([]typedCaseClause, 0, len(statement.Body.List))
	for _, item := range statement.Body.List {
		clause, ok := item.(*ast.CaseClause)
		if !ok || len(clause.List) == 0 {
			continue
		}
		caseTypes := make([]types.Type, 0, len(clause.List))
		for _, expression := range clause.List {
			caseType := pass.TypesInfo.TypeOf(expression)
			if caseType != nil && caseType != types.Typ[types.UntypedNil] {
				caseTypes = append(caseTypes, caseType)
			}
		}
		cases = append(cases, typedCaseClause{clause: clause, types: caseTypes})
	}
	return cases
}

func subsumingCaseTypes(earlier, later []types.Type) (types.Type, types.Type, bool) {
	for _, first := range earlier {
		if _, parameter := types.Unalias(first).(*types.TypeParam); parameter {
			continue
		}
		interfaceType, ok := types.Unalias(first).Underlying().(*types.Interface)
		if !ok {
			continue
		}
		interfaceType.Complete()
		for _, hidden := range later {
			if types.Implements(hidden, interfaceType) {
				return first, hidden, true
			}
		}
	}
	return nil, nil, false
}
