package rules

import (
	"fmt"
	"go/ast"
	"strings"
)

func (a *analyzer) checkConfusingNames() {
	typeNames := map[string]map[string]*ast.Ident{}
	for _, decl := range a.file.Decls {
		switch n := decl.(type) {
		case *ast.FuncDecl:
			owner := "_"
			if n.Recv != nil && len(n.Recv.List) > 0 {
				owner = exprText(n.Recv.List[0].Type)
			}
			a.checkFoldedName(typeNames, owner, n.Name)
		case *ast.GenDecl:
			for _, raw := range n.Specs {
				spec, ok := raw.(*ast.TypeSpec)
				if !ok {
					continue
				}
				structure, ok := spec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, field := range structure.Fields.List {
					for _, name := range field.Names {
						a.checkFoldedName(typeNames, spec.Name.Name, name)
					}
				}
			}
		}
	}
}

func (a *analyzer) checkFoldedName(
	groups map[string]map[string]*ast.Ident,
	owner string,
	name *ast.Ident,
) {
	if groups[owner] == nil {
		groups[owner] = map[string]*ast.Ident{}
	}
	key := strings.ToLower(name.Name)
	if first := groups[owner][key]; first != nil && first.Name != name.Name {
		a.report(
			"confusing-naming",
			name,
			fmt.Sprintf("name %s differs from %s only by capitalization", name.Name, first.Name),
		)
	} else {
		groups[owner][key] = name
	}
}
