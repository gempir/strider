package rules

import (
	"go/ast"
	"go/token"
)

func (a *analyzer) checkNoPackageVar(declaration *ast.GenDecl) {
	if declaration.Tok != token.VAR || !a.isPackageDeclaration(declaration) {
		return
	}
	for _, specNode := range declaration.Specs {
		spec, ok := specNode.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, name := range spec.Names {
			if name.Name != "_" {
				a.report(
					"no-package-var",
					name,
					"package variables introduce mutable global state",
				)
			}
		}
	}
}
