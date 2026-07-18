package semantic

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type inefficientSprintfRule struct{}

func (inefficientSprintfRule) Meta() Meta {
	return Meta{
		Code:            "inefficient-sprintf",
		Summary:         "detect fmt.Sprintf calls used only for simple conversions",
		Explanation:     "fmt.Sprintf parses a format string and uses reflection. For a single string, boolean, or base-10 integer conversion, direct strings or strconv functions preserve the output with substantially less machinery.",
		GoodExample:     "text := strconv.Itoa(number)",
		BadExample:      `text := fmt.Sprintf("%d", number)`,
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (inefficientSprintfRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				call,
					ok := node.(*ast.CallExpr)
				if !ok || call.Ellipsis.IsValid() || len(call.Args) != 2 || !isPackageFunction(
					pass.TypesInfo,
					call.Fun,
					"fmt",
					"Sprintf",
				) {
					return true
				}
				formatValue := pass.TypesInfo.Types[call.Args[0]].Value
				if formatValue == nil || formatValue.Kind() != constant.String {
					return true
				}
				replacement := simpleSprintfReplacement(
					constant.StringVal(formatValue),
					pass.TypesInfo.TypeOf(call.Args[1]),
				)
				if replacement == "" {
					return true
				}
				pass.Report(
					call,
					fmt.Sprintf(
						"fmt.Sprintf is unnecessary for this conversion; use %s",
						replacement,
					),
				)
				return true
			},
		)
	}
}

func simpleSprintfReplacement(format string, valueType types.Type) string {
	if valueType == nil {
		return ""
	}
	unaliased := types.Unalias(valueType)
	// Named basic types may implement fmt.Formatter. Restrict replacements to
	// built-in types and aliases so custom formatting behavior is preserved.
	basic, ok := unaliased.(*types.Basic)
	if !ok {
		return ""
	}

	switch format {
	case "%s":
		if basic.Info()&types.IsString != 0 {
			return "the string value directly"
		}
	case "%t":
		if basic.Kind() == types.Bool || basic.Kind() == types.UntypedBool {
			return "strconv.FormatBool"
		}
	case "%d":
		switch {
		case basic.Kind() == types.Int || basic.Kind() == types.UntypedInt:
			return "strconv.Itoa"
		case basic.Info()&types.IsInteger != 0 && basic.Info()&types.IsUnsigned != 0:
			return "strconv.FormatUint with base 10"
		case basic.Info()&types.IsInteger != 0:
			return "strconv.FormatInt with base 10"
		}
	case "%v":
		switch {
		case basic.Info()&types.IsString != 0:
			return "the string value directly"
		case basic.Kind() == types.Bool || basic.Kind() == types.UntypedBool:
			return "strconv.FormatBool"
		case basic.Kind() == types.Int || basic.Kind() == types.UntypedInt:
			return "strconv.Itoa"
		case basic.Info()&types.IsInteger != 0 && basic.Info()&types.IsUnsigned != 0:
			return "strconv.FormatUint with base 10"
		case basic.Info()&types.IsInteger != 0:
			return "strconv.FormatInt with base 10"
		}
	}
	return ""
}
