package analyze

import (
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type singleArgumentAppendRule struct {}

func (singleArgumentAppendRule) Meta() Meta {
	return Meta{
		Code: "single-argument-append",
		Summary: "detect append calls that add no elements",
		Explanation: "Calling the predeclared append function with only a slice argument returns that same slice unchanged. Assign the slice directly instead.",
		GoodExample: "destination = source",
		BadExample: "destination = append(source)",
		DefaultSeverity: diagnostic.SeverityNote,
	}
}

func (singleArgumentAppendRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				call,
				ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) != 1 || call.Ellipsis.IsValid() {
					return true
				}
				identifier,
				ok := call.Fun.(*ast.Ident)
				if !ok {
					return true
				}
				builtin,
				ok := pass.TypesInfo.ObjectOf(identifier).(*types.Builtin)
				if !ok || builtin.Name() != "append" {
					return true
				}
				pass.Report(call, "append with no elements returns the original slice unchanged")
				return true
			},
		)
	}
}
