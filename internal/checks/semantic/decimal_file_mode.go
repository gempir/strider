//strider:ignore-file cognitive-complexity
package semantic

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"

	"github.com/gempir/strider/internal/diagnostic"
)

type decimalFileModeCheck struct{}

func (decimalFileModeCheck) Meta() Meta {
	return Meta{
		Code:            "decimal-file-mode",
		Summary:         "detect decimal file modes that look like octal permissions",
		Explanation:     "Unix permission modes are conventionally written in octal. A three-digit decimal literal such as 644 passed as os.FileMode evaluates to a different bit pattern than 0o644 and is usually a missing octal prefix.",
		GoodExample:     "os.WriteFile(path, data, 0o644)",
		BadExample:      "os.WriteFile(path, data, 644)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (decimalFileModeCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
		},
		func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			for _, argument := range call.Args {
				literal, ok := argument.(*ast.BasicLit)
				if !ok || literal.Kind != token.INT || !looksLikeDecimalMode(literal.Value) || !(isNamedType(pass.TypesInfo.TypeOf(literal), "os", "FileMode") || isNamedType(
					pass.TypesInfo.TypeOf(literal),
					"io/fs",
					"FileMode",
				)) {
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
		},
	)
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
