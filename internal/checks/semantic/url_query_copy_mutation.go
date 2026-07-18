package semantic

import (
	"go/ast"

	"github.com/gempir/strider/internal/diagnostic"
)

type urlQueryCopyMutationRule struct{}

func (urlQueryCopyMutationRule) Meta() Meta {
	return Meta{
		Code:            "url-query-copy-mutation",
		Summary:         "detect mutations of the temporary copy returned by URL.Query",
		Explanation:     "URL.Query parses RawQuery and returns a new Values map. Mutating that temporary map does not update the URL unless the encoded result is assigned back to RawQuery.",
		GoodExample:     "values := address.Query(); values.Set(key, value); address.RawQuery = values.Encode()",
		BadExample:      "address.Query().Set(key, value)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (urlQueryCopyMutationRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				call,
					ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				mutation,
					ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !isURLValuesMutation(pass, mutation) {
					return true
				}
				queryCall,
					ok := ast.Unparen(mutation.X).(*ast.CallExpr)
				if !ok || len(queryCall.Args) != 0 {
					return true
				}
				querySelector,
					ok := queryCall.Fun.(*ast.SelectorExpr)
				if !ok || !isNetURLMethod(pass, querySelector, "Query") {
					return true
				}
				pass.Report(
					call,
					"URL.Query returns a copy; encode the modified values back into RawQuery",
				)
				return true
			},
		)
	}
}

func isURLValuesMutation(pass *Pass, selector *ast.SelectorExpr) bool {
	switch selector.Sel.Name {
	case "Add", "Del", "Set":
		return isNetURLMethod(pass, selector, selector.Sel.Name)
	default:
		return false
	}
}

func isNetURLMethod(pass *Pass, selector *ast.SelectorExpr, name string) bool {
	function := calledFunction(pass.TypesInfo, selector)
	return function != nil && function.Pkg() != nil && function.Pkg().Path() == "net/url" && function.Name() == name
}
