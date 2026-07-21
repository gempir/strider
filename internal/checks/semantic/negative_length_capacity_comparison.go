package semantic

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type negativeLengthCapacityComparisonCheck struct{}

func (negativeLengthCapacityComparisonCheck) Meta() Meta {
	return Meta{
		Code:            "negative-length-capacity-comparison",
		Summary:         "detect checks for negative len or cap results",
		Explanation:     "The predeclared len and cap functions always return non-negative values, so testing whether either result is below zero can never succeed.",
		GoodExample:     "if len(values) == 0 { handleEmpty() }",
		BadExample:      "if len(values) < 0 { unreachable() }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (negativeLengthCapacityComparisonCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.BinaryExpr)(nil),
		},
		func(node ast.Node) bool {
			binary, ok := node.(*ast.BinaryExpr)
			if !ok {
				return true
			}
			name := ""
			switch binary.Op {
			case token.LSS:
				if integerZero(pass, binary.Y) {
					name = lengthCapacityBuiltin(pass, binary.X)
				}
			case token.GTR:
				if integerZero(pass, binary.X) {
					name = lengthCapacityBuiltin(pass, binary.Y)
				}
			}
			if name != "" {
				pass.Report(binary, fmt.Sprintf("%s never returns a negative value", name))
			}
			return true
		},
	)
}

func lengthCapacityBuiltin(pass *Pass, expression ast.Expr) string {
	call, ok := ast.Unparen(expression).(*ast.CallExpr)
	if !ok {
		return ""
	}
	identifier, ok := call.Fun.(*ast.Ident)
	if !ok {
		return ""
	}
	builtin, ok := pass.TypesInfo.ObjectOf(identifier).(*types.Builtin)
	if !ok || builtin.Name() != "len" && builtin.Name() != "cap" {
		return ""
	}
	return builtin.Name()
}

func integerZero(pass *Pass, expression ast.Expr) bool {
	value := pass.TypesInfo.Types[expression].Value
	return value != nil && value.Kind() == constant.Int && constant.Sign(value) == 0
}

func (negativeLengthCapacityComparisonCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
