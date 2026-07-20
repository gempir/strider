package semantic

import (
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type dynamicPrintfCheck struct{}

func (dynamicPrintfCheck) Meta() Meta {
	return Meta{
		Code:            "dynamic-printf",
		Summary:         "detect Printf calls with a lone dynamic format",
		Explanation:     "Passing a dynamic string as the only format argument to a printf-style function can interpret percent signs unexpectedly. Use the print-style counterpart or an explicit %s format.",
		GoodExample:     "fmt.Printf(\"%s\", message)",
		BadExample:      "fmt.Printf(message)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (dynamicPrintfCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
			(*ast.Ident)(nil),
		},
		func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			formatIndex, ok := dynamicPrintfFormatIndex(pass, call)
			if !ok || len(call.Args) != formatIndex+1 {
				return true
			}
			format := call.Args[formatIndex]
			switch format.(type) {
			case *ast.CallExpr, *ast.Ident:
			default:
				return true
			}
			if _, tuple := pass.TypesInfo.TypeOf(format).(*types.Tuple); tuple {
				return true
			}
			pass.Report(call, "printf-style function with dynamic format string and no further arguments should use print-style function instead")
			return true
		},
	)
}

func dynamicPrintfFormatIndex(pass *Pass, call *ast.CallExpr) (int, bool) {
	function := calledFunction(pass.TypesInfo, call.Fun)
	if function == nil || function.Pkg() == nil {
		return 0, false
	}
	path, name := function.Pkg().Path(), function.Name()
	if path == "fmt" && name == "Fprintf" {
		return 1, true
	}
	if path == "fmt" && (name == "Errorf" || name == "Printf" || name == "Sprintf") {
		return 0, true
	}
	if path == "log" && (name == "Fatalf" || name == "Panicf" || name == "Printf") {
		return 0, true
	}
	if path == "testing" && (name == "Logf" || name == "Errorf" || name == "Fatalf" || name == "Skipf") {
		return 0, true
	}
	return 0, false
}

func (dynamicPrintfCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
