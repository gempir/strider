package rules

import (
	"fmt"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

func (a *Pass) checkIdentifierList(list *cst.IdentifierList) {
	for _, name := range identifierTokens(list) {
		a.checkIdentifier(name)
	}
}

func (a *Pass) checkIdentifier(name cst.Token) {
	if !name.IsValid() || name.Src() == "_" {
		return
	}
	value := name.Src()
	for _, character := range value {
		for _, banned := range a.stringsOption("characters") {
			if string(character) == banned {
				a.report("banned-characters", name, fmt.Sprintf("identifier contains banned character %q", character))
				return
			}
		}
	}
	if strings.HasPrefix(value, "_") {
		a.report("unexported-naming", name, "unexported identifier should not begin with an underscore")
	}
	if strings.Contains(value, "_") && !strings.HasPrefix(value, "Test") && !strings.HasPrefix(value, "Benchmark") && !strings.HasPrefix(value, "Example") {
		a.report("var-naming", name, "identifier should use MixedCaps rather than underscores")
	}
	if builtinIdentifiers[value] {
		a.report("redefines-builtin-id", name, fmt.Sprintf("identifier %s shadows a predeclared identifier", value))
	}
	if a.imports().names[value] {
		a.report("import-shadowing", name, fmt.Sprintf("identifier %s shadows an imported package", value))
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
	a.checkIdentifierList(field.IdentifierList)
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
		a.report("confusing-naming", name, fmt.Sprintf("name %s differs from %s only by capitalization", name.Src(), first))
	} else {
		state.foldedNames[owner][key] = name.Src()
	}
}
