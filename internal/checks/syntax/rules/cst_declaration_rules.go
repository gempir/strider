package rules

import (
	"fmt"
	"go/token"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

func (a *cstAnalyzer) finishConcreteRepeatedLiterals() {
	for literal, nodes := range a.repeatedLiterals {
		if len(nodes) > 2 {
			a.report("add-constant", nodes[2], fmt.Sprintf("string literal %s appears more than twice; define a constant", literal))
		}
	}
}

func (a *cstAnalyzer) checkConcreteTypeDefinition(definition *cst.TypeDef) {
	a.checkConcreteExportedDeclaration(definition.IDENT, definition)
	if _, ok := definition.TypeNode.(*cst.StructType); ok && token.IsExported(definition.IDENT.Src()) {
		a.publicStructs++
		if a.publicStructs > 5 {
			a.report("max-public-structs", definition.IDENT, "file declares more than 5 exported structs")
		}
	}
}

func (a *cstAnalyzer) checkConcreteExportedFunction(name cst.Token, node cst.Node, method bool) {
	if !token.IsExported(name.Src()) || a.packageName == "main" {
		return
	}
	if strings.HasSuffix(a.filename, "_test.go") && (strings.HasPrefix(name.Src(), "Test") || strings.HasPrefix(name.Src(), "Benchmark") || strings.HasPrefix(
		name.Src(),
		"Example",
	)) {
		return
	}
	if method {
		switch name.Src() {
		case "Error", "Read", "ServeHTTP", "String", "Write", "Unwrap":
			return
		}
	}
	if !a.concreteHasDocumentation(name.Src(), node) {
		a.report("exported", name, "exported function or method should have a comment beginning with its name")
	}
}

func (a *cstAnalyzer) checkConcreteExportedDeclaration(name cst.Token, node cst.Node) {
	if !token.IsExported(name.Src()) || a.concreteHasDocumentation(name.Src(), node) {
		return
	}
	message := "exported declaration should have a comment beginning with its name"
	if _, ok := node.(*cst.TypeDef); ok {
		message = "exported type should have a comment beginning with its name"
	}
	a.report("exported", name, message)
}

func (a *cstAnalyzer) checkConcreteExportedList(list *cst.IdentifierList, node cst.Node) {
	for _, name := range concreteIdentifierTokens(list) {
		a.checkConcreteExportedDeclaration(name, node)
	}
}

func (a *cstAnalyzer) concreteHasDocumentation(name string, node cst.Node) bool {
	start, _ := cst.Range(node)
	source := a.tree.Bytes()
	comments := a.tree.Comments()
	for index := len(comments) - 1; index >= 0; index-- {
		comment := comments[index]
		if comment.End > start {
			continue
		}
		if strings.Count(string(source[comment.End:start]), "\n") > 1 {
			return false
		}
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(comment.Text, "//"), "/*"))
		text = strings.TrimSpace(strings.TrimSuffix(text, "*/"))
		return strings.HasPrefix(text, name)
	}
	return false
}

func (a *cstAnalyzer) checkConcreteVarSpec(name cst.Token, typeNode cst.Node, values *cst.ExpressionList) {
	a.checkConcreteExportedDeclaration(name, name)
	typeName := cst.Spelling(typeNode)
	if a.concretePackageDeclaration() && concreteValueIsError(typeName, values) && !strings.HasPrefix(name.Src(), "err") && !strings.HasPrefix(name.Src(), "Err") {
		a.report("error-naming", name, "package error variable should be named errFoo or ErrFoo")
	}
	if typeNode != nil && values != nil && values.Len() == 1 && concreteZeroValue(values.Expression) {
		a.report("var-declaration", values.Expression, "omit the explicit zero value from the variable declaration")
	}
	if typeName == "time.Duration" && hasTimeUnitSuffix(name.Src()) {
		a.report("time-naming", name, "time.Duration name should not include a unit suffix")
	}
}

func (a *cstAnalyzer) checkConcreteVarSpecList(names *cst.IdentifierList, typeNode cst.Node, values *cst.ExpressionList) {
	for _, name := range concreteIdentifierTokens(names) {
		a.checkConcreteVarSpec(name, typeNode, values)
	}
}

func (a *cstAnalyzer) concretePackageDeclaration() bool {
	for _, ancestor := range a.ancestors {
		switch ancestor.(type) {
		case *cst.FunctionDecl, *cst.MethodDecl, *cst.FunctionLit:
			return false
		}
	}
	return true
}

func concreteValueIsError(typeName string, values *cst.ExpressionList) bool {
	if strings.HasPrefix(typeName, "error") {
		return true
	}
	if values == nil {
		return false
	}
	call, ok := values.Expression.(*cst.PrimaryExpr)
	name := concreteCallName(call)
	return ok && (name == "errors.New" || name == "fmt.Errorf")
}

func concreteZeroValue(node cst.Node) bool {
	spelling := cst.Spelling(node)
	return spelling == "nil" || spelling == "false" || spelling == "0" || spelling == `""` || spelling == "''"
}
