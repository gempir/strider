package semantic

import (
	"go/ast"
	"go/types"
	"strings"

	"github.com/gempir/strider/internal/diagnostic"
)

type errorTypeNamingCheck struct{}

func (errorTypeNamingCheck) Meta() Meta {
	return Meta{
		Code:            "error-type-naming",
		Summary:         "name error implementations with an Error suffix",
		Explanation:     "A named type whose value or pointer method set implements error should use an Error suffix so its role is recognizable at API boundaries and in type assertions.",
		GoodExample:     "type ParseError struct { Offset int }",
		BadExample:      "type ParseFailure struct { Offset int } // implements Error() string",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (errorTypeNamingCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.TypeSpec)(nil),
		},
		func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok || spec.Assign.IsValid() || strings.HasSuffix(spec.Name.Name, "Error") {
				return true
			}
			object, _ := pass.TypesInfo.Defs[spec.Name].(*types.TypeName)
			if object == nil {
				return true
			}
			valueType := object.Type()
			errorInterface, _ := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
			if types.Implements(valueType, errorInterface) || types.Implements(types.NewPointer(valueType), errorInterface) {
				pass.Report(spec.Name, "error implementation type should have an Error suffix")
			}
			return true
		},
	)
}
