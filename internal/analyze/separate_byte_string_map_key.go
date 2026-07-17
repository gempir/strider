package analyze

import (
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type separateByteStringMapKeyRule struct {}

func (separateByteStringMapKeyRule) Meta() Meta {
	return Meta{
		Code: "separate-byte-string-map-key",
		Summary: "detect allocated byte-to-string temporaries used only for map lookups",
		Explanation: "The compiler can perform m[string(bytes)] without copying the byte slice because the temporary string cannot escape the lookup. Assigning string(bytes) to a variable first prevents that optimization and allocates when the variable is used only as a map key.",
		GoodExample: "value := items[string(keyBytes)]",
		BadExample: "key := string(keyBytes); value := items[key]",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (separateByteStringMapKeyRule) Run(pass *Pass) {
	parents := analysisParents(pass.Files)
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				switch declaration := node.(type) {
				case *ast.AssignStmt:
					if declaration.Tok.String() != ":=" || len(declaration.Lhs) != len(
						declaration.Rhs,
					) {
						return true
					}
					for index,
					left := range declaration.Lhs {
						identifier,
						ok := left.(*ast.Ident)
						if !ok {
							continue
						}
						reportSeparateMapKey(pass, parents, identifier, declaration.Rhs[index])
					}
				case *ast.ValueSpec:
					if len(declaration.Names) != len(declaration.Values) {
						return true
					}
					for index,
					identifier := range declaration.Names {
						reportSeparateMapKey(pass, parents, identifier, declaration.Values[index])
					}
				}
				return true
			},
		)
	}
}

func reportSeparateMapKey(
	pass *Pass,
	parents map[ast.Node]ast.Node,
	identifier *ast.Ident,
	expression ast.Expr,
) {
	object := pass.TypesInfo.Defs[identifier]
	conversion, ok := byteSliceStringConversion(pass, expression)
	if object == nil || !ok || !usedOnlyAsStringMapKey(pass, parents, object) {
		return
	}
	pass.Report(
		conversion,
		"inline this byte-to-string conversion in each map lookup to avoid an allocation",
	)
}

func byteSliceStringConversion(pass *Pass, expression ast.Expr) (*ast.CallExpr, bool) {
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	identifier, ok := call.Fun.(*ast.Ident)
	if !ok || identifier.Name != "string" {
		return nil, false
	}
	if _, ok := pass.TypesInfo.Uses[identifier].(*types.TypeName); !ok {
		return nil, false
	}
	slice, ok := pass.TypesInfo.TypeOf(call.Args[0]).Underlying().(*types.Slice)
	if !ok || !isByteType(slice.Elem()) {
		return nil, false
	}
	return call, true
}

func usedOnlyAsStringMapKey(pass *Pass, parents map[ast.Node]ast.Node, object types.Object) bool {
	uses := 0
	for identifier, usedObject := range pass.TypesInfo.Uses {
		if usedObject != object {
			continue
		}
		uses++
		index, ok := parents[identifier].(*ast.IndexExpr)
		if !ok || index.Index != identifier {
			return false
		}
		mapping, ok := pass.TypesInfo.TypeOf(index.X).Underlying().(*types.Map)
		if !ok || !types.Identical(mapping.Key(), types.Typ[types.String]) {
			return false
		}
	}
	return uses != 0
}
