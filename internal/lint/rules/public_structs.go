package rules

import (
	"go/ast"
	"go/token"
)

func (a *analyzer) checkPublicStructCount() {
	count := 0
	for _, decl := range a.file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, raw := range gen.Specs {
			spec := raw.(*ast.TypeSpec)
			if ast.IsExported(spec.Name.Name) {
				if _, ok := spec.Type.(*ast.StructType); ok {
					count++
					if count > 5 {
						a.report(
							"max-public-structs",
							spec.Name,
							"file declares more than 5 exported structs",
						)
					}
				}
			}
		}
	}
}
