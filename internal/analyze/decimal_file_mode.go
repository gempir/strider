package analyze

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/gempir/strider/internal/diagnostic"
)

type decimalFileModeRule struct{}

func (decimalFileModeRule) Meta() Meta {
	return Meta{
		Code:            "decimal-file-mode",
		Summary:         "detect decimal file modes that look like octal permissions",
		Explanation:     "Unix permission modes are conventionally written in octal. A three-digit decimal literal such as 644 passed as os.FileMode evaluates to a different bit pattern than 0o644 and is usually a missing octal prefix.",
		GoodExample:     "os.WriteFile(path, data, 0o644)",
		BadExample:      "os.WriteFile(path, data, 644)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (decimalFileModeRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			for _, argument := range call.Args {
				literal, ok := argument.(*ast.BasicLit)
				if !ok || literal.Kind != token.INT || !looksLikeDecimalMode(literal.Value) ||
					!isFileModeType(pass.TypesInfo.TypeOf(literal)) {
					continue
				}
				value, err := strconv.ParseInt(literal.Value, 10, 64)
				if err != nil {
					continue
				}
				pass.Report(
					literal,
					fmt.Sprintf("decimal file mode %s evaluates to %#o; use 0o%s if octal permissions were intended", literal.Value, value, literal.Value),
				)
			}
			return true
		})
	}
}

func looksLikeDecimalMode(value string) bool {
	if len(value) != 3 || value[0] == '0' {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '7' {
			return false
		}
	}
	return true
}

func isFileModeType(valueType types.Type) bool {
	named, ok := types.Unalias(valueType).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Name() != "FileMode" {
		return false
	}
	path := named.Obj().Pkg().Path()
	return path == "os" || path == "io/fs"
}
