package semantic

import (
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type byteStringWriteCheck struct{}

func (byteStringWriteCheck) Meta() Meta {
	return Meta{
		Code:            "byte-string-write",
		Summary:         "detect byte slices converted to strings for io.WriteString",
		Explanation:     "io.WriteString(writer, string(bytes)) allocates and copies the byte slice before writing it. The writer already accepts bytes through Write, so write the original slice directly.",
		GoodExample:     "_, err := writer.Write(bytes)",
		BadExample:      "_, err := io.WriteString(writer, string(bytes))",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (byteStringWriteCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
		},
		func(node ast.Node) bool {
			call,
				ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 || !isPackageFunction(pass.TypesInfo, call.Fun, "io", "WriteString") {
				return true
			}
			conversion,
				ok := call.Args[1].(*ast.CallExpr)
			if !ok || len(conversion.Args) != 1 {
				return true
			}
			identifier,
				ok := conversion.Fun.(*ast.Ident)
			if !ok || identifier.Name != "string" {
				return true
			}
			if _,
				ok := pass.TypesInfo.Uses[identifier].(*types.TypeName); !ok {
				return true
			}
			slice,
				ok := pass.TypesInfo.TypeOf(conversion.Args[0]).Underlying().(*types.Slice)
			if !ok || !isByteType(slice.Elem()) {
				return true
			}
			pass.Report(call, "write the byte slice directly instead of converting it for io.WriteString")
			return true
		},
	)
}

func (byteStringWriteCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
