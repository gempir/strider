package syntax

import (
	"fmt"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

func (a *Pass) checkBannedCharacters(name cst.Token) {
	if !name.IsValid() || name.Src() == "_" {
		return
	}
	for _, character := range name.Src() {
		for _, banned := range a.stringsOption("characters") {
			if string(character) == banned {
				a.Report(name, fmt.Sprintf("identifier contains banned character %q", character))
				return
			}
		}
	}
}

func (a *Pass) checkUnexportedNaming(name cst.Token) {
	if !name.IsValid() || name.Src() == "_" {
		return
	}
	value := name.Src()
	if strings.HasPrefix(value, "_") {
		a.Report(name, "unexported identifier should not begin with an underscore")
	}
}

func (a *Pass) checkVarNaming(name cst.Token) {
	if !name.IsValid() || name.Src() == "_" {
		return
	}
	value := name.Src()
	if strings.Contains(value, "_") && !strings.HasPrefix(value, "Test") && !strings.HasPrefix(value, "Benchmark") && !strings.HasPrefix(value, "Example") {
		a.Report(name, "identifier should use MixedCaps rather than underscores")
	}
}

func (a *Pass) checkRedefinesBuiltin(name cst.Token) {
	if !name.IsValid() || name.Src() == "_" {
		return
	}
	value := name.Src()
	if predeclaredIdentifier(value) {
		a.Report(name, fmt.Sprintf("identifier %s shadows a predeclared identifier", value))
	}
}

func (a *Pass) checkImportShadowing(name cst.Token) {
	if !name.IsValid() || name.Src() == "_" {
		return
	}
	value := name.Src()
	if a.imports().names[value] {
		a.Report(name, fmt.Sprintf("identifier %s shadows an imported package", value))
	}
}

func (a *Pass) checkMethodName(method *cst.MethodDecl) {
	declarations := parameterDecls(method.Receiver)
	owner := "_"
	if len(declarations) != 0 {
		owner = cst.Spelling(declarations[0].TypeNode)
	}
	a.checkFoldedName(owner, method.MethodName)
}

func (a *Pass) checkFieldNames(field *cst.FieldDecl) {
	owner := ""
	for index := len(a.ancestors) - 1; index >= 0; index-- {
		if definition, ok := a.ancestors[index].(*cst.TypeDef); ok {
			owner = definition.IDENT.Src()
			break
		}
	}
	if owner == "" {
		return
	}
	for _, name := range identifierTokens(field.IdentifierList) {
		a.checkFoldedName(owner, name)
	}
}

func (a *Pass) checkFoldedName(owner string, name cst.Token) {
	if !name.IsValid() {
		return
	}
	state := a.namingState()
	if state.foldedNames[owner] == nil {
		state.foldedNames[owner] = map[string]string{}
	}
	key := strings.ToLower(name.Src())
	if first := state.foldedNames[owner][key]; first != "" && first != name.Src() {
		a.Report(name, fmt.Sprintf("name %s differs from %s only by capitalization", name.Src(), first))
	} else {
		state.foldedNames[owner][key] = name.Src()
	}
}
