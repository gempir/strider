package semantic

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"net/http"

	"github.com/gempir/strider/internal/diagnostic"
)

type nonCanonicalHeaderRule struct{}

func (nonCanonicalHeaderRule) Meta() Meta {
	return Meta{
		Code:            "non-canonical-header",
		Summary:         "detect non-canonical keys in http.Header reads",
		Explanation:     "Direct reads from http.Header must use canonical constant keys. Header methods canonicalize their arguments, but direct map access does not.",
		GoodExample:     `value := header["Content-Type"]`,
		BadExample:      `value := header["content-type"]`,
		DefaultSeverity: diagnostic.SeverityNote,
	}
}

func (nonCanonicalHeaderRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				if assignment,
					ok := node.(*ast.AssignStmt); ok {
					for _, expression := range assignment.Lhs {
						index,
							ok := expression.(*ast.IndexExpr)
						if ok && isHTTPHeader(pass.TypesInfo.TypeOf(index.X)) {
							return false
						}
					}
					return true
				}
				index,
					ok := node.(*ast.IndexExpr)
				if !ok || !isHTTPHeader(pass.TypesInfo.TypeOf(index.X)) {
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
				pass.Report(
					index,
					fmt.Sprintf(
						"keys in http.Header are canonicalized, %q is not canonical; fix the constant or use http.CanonicalHeaderKey",
						key,
					),
				)
				return true
			},
		)
	}
}

func isHTTPHeader(valueType types.Type) bool {
	named, ok := types.Unalias(valueType).(*types.Named)
	return ok && named.Obj() != nil && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "net/http" && named.Obj().Name() == "Header"
}
