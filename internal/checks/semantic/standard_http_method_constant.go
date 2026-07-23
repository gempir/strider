//strider:ignore-file cyclomatic-complexity,no-package-var,single-case-switch
package semantic

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"

	"github.com/gempir/strider/internal/diagnostic"
)

var standardHTTPMethods = map[string]string{
	"CONNECT": "MethodConnect",
	"DELETE":  "MethodDelete",
	"GET":     "MethodGet",
	"HEAD":    "MethodHead",
	"OPTIONS": "MethodOptions",
	"PATCH":   "MethodPatch",
	"POST":    "MethodPost",
	"PUT":     "MethodPut",
	"TRACE":   "MethodTrace",
}

type standardHTTPMethodConstantCheck struct{}

func (standardHTTPMethodConstantCheck) Meta() Meta {
	return Meta{
		Code:            "standard-http-method-constant",
		Summary:         "prefer net/http method constants",
		Explanation:     "Using net/http method constants avoids spelling drift and makes the protocol role of an argument explicit. This check is limited to method arguments of net/http request constructors.",
		GoodExample:     "http.NewRequest(http.MethodGet, endpoint, nil)",
		BadExample:      "http.NewRequest(\"GET\", endpoint, nil)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (standardHTTPMethodConstantCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
		},
		func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			function := calledFunction(pass.TypesInfo, call.Fun)
			if function == nil || function.Pkg() == nil || function.Pkg().Path() != "net/http" {
				return true
			}
			argument := -1
			switch function.Name() {
			case "NewRequest":
				argument = 0
			case "NewRequestWithContext":
				argument = 1
			}
			if argument < 0 || argument >= len(call.Args) {
				return true
			}
			if standardHTTPMethodObject(pass, call.Args[argument]) {
				return true
			}
			value := pass.TypesInfo.Types[call.Args[argument]].Value
			if value == nil || value.Kind() != constant.String {
				return true
			}
			method := constant.StringVal(value)
			name := standardHTTPMethods[method]
			if name != "" {
				pass.Report(call.Args[argument], "replace the HTTP method literal with http."+name)
			}
			return true
		},
	)
}

func standardHTTPMethodObject(pass *Pass, expression ast.Expr) bool {
	var object types.Object
	switch expression := ast.Unparen(expression).(type) {
	case *ast.Ident:
		object = pass.TypesInfo.ObjectOf(expression)
	case *ast.SelectorExpr:
		object = pass.TypesInfo.ObjectOf(expression.Sel)
	}
	constantObject, _ := object.(*types.Const)
	return constantObject != nil && constantObject.Pkg() != nil && constantObject.Pkg().Path() == "net/http" && strings.HasPrefix(constantObject.Name(), "Method")
}
