package rules

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

func (a *analyzer) checkUseAny(iface *ast.InterfaceType) {
	if iface.Methods != nil && len(iface.Methods.List) == 0 {
		a.report("use-any", iface, "use any instead of interface{}")
	}
}

func (a *analyzer) checkLiteral(literal *ast.BasicLit) {
	if literal.Kind != token.STRING {
		return
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return
	}
	lower := strings.ToLower(value)
	for _, scheme := range []string{"http://", "ws://", "ftp://"} {
		if strings.HasPrefix(lower, scheme) {
			a.report(
				"unsecure-url-scheme",
				literal,
				fmt.Sprintf("URL uses insecure %s scheme", strings.TrimSuffix(scheme, "://")),
			)
			break
		}
	}
}

func (a *analyzer) checkReturn(statement *ast.ReturnStmt) {
	a.checkNoNakedReturn(statement)
	if len(statement.Results) != 0 {
		return
	}
	for index := len(a.ancestors) - 1; index >= 0; index-- {
		var results *ast.FieldList
		switch fn := a.ancestors[index].(type) {
		case *ast.FuncDecl:
			results = fn.Type.Results
		case *ast.FuncLit:
			results = fn.Type.Results
		default:
			continue
		}
		if results != nil {
			for _, field := range results.List {
				if len(field.Names) > 0 {
					a.report(
						"bare-return",
						statement,
						"avoid bare returns; add explicit return expressions",
					)
					return
				}
			}
		}
		return
	}
}
