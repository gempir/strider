package semantic

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

const interfaceMethodLimit = 10

type interfaceMethodLimitRule struct{}

func (interfaceMethodLimitRule) Meta() Meta {
	return Meta{
		Code:            "interface-method-limit",
		Summary:         "detect interfaces with more than 10 methods",
		Explanation:     "Interfaces are easiest to implement, compose, and test when they remain small. This check uses a documented limit of 10 methods, including methods contributed by embedded interfaces, to identify abstractions that may need to be split by responsibility.",
		GoodExample:     "type Reader interface { Read([]byte) (int, error) }",
		BadExample:      "type Service interface { Start(); Stop(); Pause(); Resume(); Reload(); Status(); Health(); Metrics(); Configure(); Validate(); Reset() }",
		DefaultSeverity: diagnostic.SeverityNote,
	}
}

func (interfaceMethodLimitRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				declaration,
					ok := node.(*ast.InterfaceType)
				if !ok {
					return true
				}
				declaredType := pass.TypesInfo.TypeOf(declaration)
				if declaredType == nil {
					return true
				}
				interfaceType,
					ok := types.Unalias(declaredType).Underlying().(*types.Interface)
				if !ok {
					return true
				}
				interfaceType.Complete()
				methodCount := interfaceType.NumMethods()
				if methodCount <= interfaceMethodLimit {
					return true
				}
				pass.Report(
					declaration,
					fmt.Sprintf(
						"interface has %d methods, exceeding the configured design limit of %d",
						methodCount,
						interfaceMethodLimit,
					),
				)
				return true
			},
		)
	}
}
