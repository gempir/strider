package semantic

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/checks/catalog"
	"github.com/gempir/strider/internal/diagnostic"
)

type interfaceMethodLimitCheck struct{}

func (interfaceMethodLimitCheck) Meta() Meta {
	return Meta{
		Code:            "interface-method-limit",
		Summary:         "limit interface method count",
		Explanation:     "Interfaces are easiest to implement, compose, and test when they remain small. The built-in maximum is 10 methods, including methods contributed by embedded interfaces; max-methods can override it for a project.",
		GoodExample:     "type Reader interface { Read([]byte) (int, error) }",
		BadExample:      "type Service interface { Start(); Stop(); Pause(); Resume(); Reload(); Status(); Health(); Metrics(); Configure(); Validate(); Reset() }",
		DefaultSeverity: diagnostic.SeverityWarning,
		Options: []catalog.Option{
			catalog.NonNegativeIntOption("max-methods", 10, "Maximum number of methods allowed on an interface, including embedded methods."),
		},
	}
}

func (interfaceMethodLimitCheck) Run(pass *Pass) {
	limit := pass.IntOption("max-methods")
	pass.Inspect(
		[]ast.Node{
			(*ast.InterfaceType)(nil),
		},
		func(node ast.Node) bool {
			declaration, ok := node.(*ast.InterfaceType)
			if !ok {
				return true
			}
			declaredType := pass.TypesInfo.TypeOf(declaration)
			if declaredType == nil {
				return true
			}
			interfaceType, ok := types.Unalias(declaredType).Underlying().(*types.Interface)
			if !ok {
				return true
			}
			interfaceType.Complete()
			methodCount := interfaceType.NumMethods()
			if methodCount <= limit {
				return true
			}
			pass.Report(declaration, fmt.Sprintf("interface has %d methods, exceeding the configured design limit of %d", methodCount, limit))
			return true
		},
	)
}

func (interfaceMethodLimitCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
