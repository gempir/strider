package semantic

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type contradictoryInterfaceAssertionRule struct{}

func (contradictoryInterfaceAssertionRule) Meta() Meta {
	return Meta{
		Code:            "contradictory-interface-assertion",
		Summary:         "detect interface assertions with conflicting method signatures",
		Explanation:     "An assertion from one interface to another can compile even when the two method sets contain a same-named method with incompatible signatures. No dynamic type can implement both contracts, so the assertion can never succeed.",
		GoodExample:     "value, ok := source.(interface { Write([]byte) error })",
		BadExample:      "value, ok := source.(interface { Read() string }) // source requires Read() int",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (contradictoryInterfaceAssertionRule) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.TypeAssertExpr)(nil),
		},
		func(node ast.Node) bool {
			assertion,
				ok := node.(*ast.TypeAssertExpr)
			if !ok || assertion.Type == nil {
				return true
			}
			left := pass.TypesInfo.TypeOf(assertion.X)
			right := pass.TypesInfo.TypeOf(assertion.Type)
			rightInterface,
				ok := right.Underlying().(*types.Interface)
			if !ok {
				return true
			}
			leftMethods := types.NewMethodSet(left)
			for method := range rightInterface.Methods() {
				selection := leftMethods.Lookup(method.Pkg(), method.Name())
				if selection == nil {
					continue
				}
				leftMethod,
					ok := selection.Obj().(*types.Func)
				if !ok || leftMethod.Origin() != leftMethod || method.Origin() != method {
					return true
				}
				if types.AssignableTo(leftMethod.Type(), method.Type()) {
					continue
				}
				pass.Report(
					assertion,
					fmt.Sprintf(
						"interface assertion can never succeed: method %s has type %s in the source interface and %s in the asserted interface",
						method.Name(),
						types.TypeString(leftMethod.Type(), types.RelativeTo(pass.Types)),
						types.TypeString(method.Type(), types.RelativeTo(pass.Types)),
					),
				)
				return true
			}
			return true
		},
	)
}

func (contradictoryInterfaceAssertionRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
