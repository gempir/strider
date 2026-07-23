package semantic

import (
	"fmt"
	"go/ast"
	"go/constant"
	"net/http"

	"github.com/gempir/strider/internal/diagnostic"
)

type nonCanonicalHeaderCheck struct{}

func (nonCanonicalHeaderCheck) Meta() Meta {
	return Meta{
		Code:            "non-canonical-header",
		Summary:         "detect non-canonical keys in http.Header reads",
		Explanation:     "Direct reads from http.Header must use canonical constant keys. Header methods canonicalize their arguments, but direct map access does not.",
		GoodExample:     `value := header["Content-Type"]`,
		BadExample:      `value := header["content-type"]`,
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (nonCanonicalHeaderCheck) Run(pass *Pass) {
	assigned := make(map[*ast.IndexExpr]bool)
	pass.Inspect(
		[]ast.Node{
			(*ast.AssignStmt)(nil),
			(*ast.IndexExpr)(nil),
		},
		func(node ast.Node) bool {
			if assignment, ok := node.(*ast.AssignStmt); ok {
				for _, expression := range assignment.Lhs {
					index, ok := ast.Unparen(expression).(*ast.IndexExpr)
					if ok && isNamedType(pass.TypesInfo.TypeOf(index.X), "net/http", "Header") {
						assigned[index] = true
					}
				}
				return true
			}
			index, ok := node.(*ast.IndexExpr)
			if !ok || assigned[index] || !isNamedType(pass.TypesInfo.TypeOf(index.X), "net/http", "Header") {
				return true
			}
			value := pass.TypesInfo.Types[index.Index].Value
			if value == nil || value.Kind() != constant.String {
				return true
			}
			key := constant.StringVal(value)
			if key == http.CanonicalHeaderKey(key) {
				return true
			}
			pass.Report(index, fmt.Sprintf("keys in http.Header are canonicalized, %q is not canonical; fix the constant or use http.CanonicalHeaderKey", key))
			return true
		},
	)
}
