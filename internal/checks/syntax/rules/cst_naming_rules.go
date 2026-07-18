package rules

import (
	"fmt"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

func (a *cstAnalyzer) checkConcreteIdentifierList(list *cst.IdentifierList) {
	for _, name := range concreteIdentifierTokens(list) {
		a.checkConcreteIdentifier(name)
	}
}

func (a *cstAnalyzer) checkConcreteIdentifier(name cst.Token) {
	if !name.IsValid() || name.Src() == "_" {
		return
	}
	value := name.Src()
	if strings.HasPrefix(value, "_") {
		a.report("unexported-naming", name, "unexported identifier should not begin with an underscore")
	}
	if strings.Contains(value, "_") && !strings.HasPrefix(value, "Test") && !strings.HasPrefix(value, "Benchmark") && !strings.HasPrefix(value, "Example") {
		a.report("var-naming", name, "identifier should use MixedCaps rather than underscores")
	}
	if builtinIdentifiers[value] {
		a.report("redefines-builtin-id", name, fmt.Sprintf("identifier %s shadows a predeclared identifier", value))
	}
	if a.importNames[value] {
		a.report("import-shadowing", name, fmt.Sprintf("identifier %s shadows an imported package", value))
	}
}

func (a *cstAnalyzer) checkConcreteMethodName(method *cst.MethodDecl) {
	declarations := concreteParameterDecls(method.Receiver)
	owner := "_"
	if len(declarations) != 0 {
		owner = cst.Spelling(declarations[0].TypeNode)
	}
	a.checkConcreteFoldedName(owner, method.MethodName)
}

func (a *cstAnalyzer) checkConcreteFieldNames(field *cst.FieldDecl) {
	a.checkConcreteIdentifierList(field.IdentifierList)
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
	for _, name := range concreteIdentifierTokens(field.IdentifierList) {
		a.checkConcreteFoldedName(owner, name)
	}
}

func (a *cstAnalyzer) checkConcreteFoldedName(owner string, name cst.Token) {
	if !name.IsValid() {
		return
	}
	if a.foldedNames[owner] == nil {
		a.foldedNames[owner] = map[string]string{}
	}
	key := strings.ToLower(name.Src())
	if first := a.foldedNames[owner][key]; first != "" && first != name.Src() {
		a.report("confusing-naming", name, fmt.Sprintf("name %s differs from %s only by capitalization", name.Src(), first))
	} else {
		a.foldedNames[owner][key] = name.Src()
	}
}
