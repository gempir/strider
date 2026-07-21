package semantic

import (
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type nilContextCheck struct{}

func (nilContextCheck) Meta() Meta {
	return Meta{
		Code:            "nil-context",
		Summary:         "detect nil context.Context arguments",
		Explanation:     "A context.Context must not be nil. Pass context.TODO when the appropriate parent is not yet known, or context.Background for an explicit root context.",
		GoodExample:     "load(context.TODO())",
		BadExample:      "load(nil)",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (nilContextCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
		},
		func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 || !isNilIdentifier(call.Args[0]) {
				return true
			}
			function := calledFunction(pass.TypesInfo, call.Fun)
			if function == nil {
				return true
			}
			signature, ok := function.Type().(*types.Signature)
			if !ok || signature.Params().Len() == 0 || !isNamedType(signature.Params().At(0).Type(), "context", "Context") {
				return true
			}
			pass.Report(call.Args[0], "do not pass a nil Context, even if a function permits it; pass context.TODO if you are unsure about which Context to use")
			return true
		},
	)
}

func isNilIdentifier(expression ast.Expr) bool {
	identifier, ok := expression.(*ast.Ident)
	return ok && identifier.Name == "nil"
}

func (nilContextCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
