package rules

import (
	"go/ast"
	"go/token"
	"strings"
)

func (a *analyzer) checkDeclaration(decl *ast.GenDecl) {
	a.checkNoPackageVar(decl)
	if decl.Tok == token.IMPORT {
		return
	}
	if decl.Doc != nil && strings.Contains(decl.Doc.Text(), "Code generated") {
		return
	}
	for _, raw := range decl.Specs {
		switch spec := raw.(type) {
		case *ast.TypeSpec:
			a.checkIdentifierName(spec.Name)
			if ast.IsExported(spec.Name.Name) &&
				(decl.Doc == nil ||
					!strings.HasPrefix(strings.TrimSpace(decl.Doc.Text()), spec.Name.Name)) {
				a.report(
					"exported",
					spec.Name,
					"exported type should have a comment beginning with its name",
				)
			}
		case *ast.ValueSpec:
			a.checkValueSpec(decl, spec)
		}
	}
}

func (a *analyzer) checkValueSpec(decl *ast.GenDecl, spec *ast.ValueSpec) {
	for index, name := range spec.Names {
		a.checkIdentifierName(name)
		if ast.IsExported(name.Name) &&
			(decl.Doc == nil || !strings.HasPrefix(strings.TrimSpace(decl.Doc.Text()), name.Name)) {
			a.report(
				"exported",
				name,
				"exported declaration should have a comment beginning with its name",
			)
		}
		if decl.Tok == token.VAR && a.isPackageDeclaration(decl) &&
			strings.HasPrefix(typeAt(spec, index), "error") &&
			!strings.HasPrefix(name.Name, "err") &&
			!strings.HasPrefix(name.Name, "Err") {
			a.report(
				"error-naming",
				name,
				"package error variable should be named errFoo or ErrFoo",
			)
		}
		if decl.Tok == token.VAR && spec.Type != nil && len(spec.Values) == 1 &&
			isZeroValue(spec.Values[0]) {
			a.report(
				"var-declaration",
				spec.Values[0],
				"omit the explicit zero value from the variable declaration",
			)
		}
		if exprText(spec.Type) == "time.Duration" && hasTimeUnitSuffix(name.Name) {
			a.report("time-naming", name, "time.Duration name should not include a unit suffix")
		}
	}
}

func (a *analyzer) isPackageDeclaration(node ast.Node) bool {
	for _, ancestor := range a.ancestors {
		if ancestor == a.file {
			continue
		}
		if _, ok := ancestor.(*ast.FuncDecl); ok {
			return false
		}
	}
	return true
}

func typeAt(spec *ast.ValueSpec, index int) string {
	if spec.Type != nil {
		return exprText(spec.Type)
	}
	if index < len(spec.Values) {
		if call, ok := spec.Values[index].(*ast.CallExpr); ok {
			name := callName(call)
			if name == "errors.New" || name == "fmt.Errorf" {
				return "error"
			}
		}
	}
	return ""
}

func isZeroValue(expr ast.Expr) bool {
	switch n := expr.(type) {
	case *ast.Ident:
		return n.Name == "nil" || n.Name == "false"
	case *ast.BasicLit:
		return n.Value == "0" || n.Value == `""` || n.Value == "''"
	}
	return false
}
