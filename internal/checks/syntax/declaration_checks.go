//strider:ignore-file single-case-switch
package syntax

import (
	"fmt"
	"go/token"
	"strconv"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

func (a *Pass) finishRepeatedLiterals() {
	for literal, nodes := range a.repeatedLiteralState().literals {
		if len(nodes) > 2 {
			a.Report(nodes[2], fmt.Sprintf("string literal %s appears more than twice; define a constant", literal))
		}
	}
}

func (a *Pass) checkMaxPublicStructs(definition *cst.TypeDef) {
	if _, ok := definition.TypeNode.(*cst.StructType); ok && token.IsExported(definition.IDENT.Src()) {
		state := a.declarationState()
		state.publicStructs++
		limit := a.intOption("max-public-structs")
		if state.publicStructs > limit {
			a.Report(definition.IDENT, fmt.Sprintf("file declares more than %d exported structs", limit))
		}
	}
}

func (a *Pass) checkExportedFunction(name cst.Token, node cst.Node, method bool) {
	if !token.IsExported(name.Src()) || a.packageNameToken().Src() == "main" {
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
	if !a.hasDocumentation(name.Src(), node) {
		a.Report(name, "exported function or method should have a comment beginning with its name")
	}
}

func (a *Pass) observeRepeatedLiteral(literal *cst.BasicLit, ancestors []cst.Node) {
	if literal.Ch() != token.STRING {
		return
	}
	for _, ancestor := range ancestors {
		switch cst.Kind(ancestor) {
		case "ConstDecl", "VarDecl", "TypeDecl":
			return
		}
	}
	value, err := strconv.Unquote(literal.Src())
	if err != nil || value == "" {
		return
	}
	state := a.repeatedLiteralState()
	state.literals[literal.Src()] = append(state.literals[literal.Src()], literal)
}

func (a *Pass) checkExportedDeclaration(name cst.Token, node cst.Node) {
	if !token.IsExported(name.Src()) || a.hasDocumentation(name.Src(), node) {
		return
	}
	message := "exported declaration should have a comment beginning with its name"
	if _, ok := node.(*cst.TypeDef); ok {
		message = "exported type should have a comment beginning with its name"
	}
	a.Report(name, message)
}

func (a *Pass) checkExportedList(list *cst.IdentifierList, node cst.Node) {
	for _, name := range identifierTokens(list) {
		a.checkExportedDeclaration(name, node)
	}
}

func (a *Pass) hasDocumentation(name string, node cst.Node) bool {
	start, _ := a.tree.Range(node)
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

func (a *Pass) checkErrorNaming(name cst.Token, typeNode cst.Node, values *cst.ExpressionList) {
	typeName := cst.Spelling(typeNode)
	if a.packageDeclaration() && valueIsError(typeName, values) && !strings.HasPrefix(name.Src(), "err") && !strings.HasPrefix(name.Src(), "Err") {
		a.Report(name, "package error variable should be named errFoo or ErrFoo")
	}
}

func (a *Pass) checkVarDeclaration(_ cst.Token, typeNode cst.Node, values *cst.ExpressionList) {
	if typeNode != nil && values != nil && values.Len() == 1 && zeroValue(values.Expression) {
		a.Report(values.Expression, "omit the explicit zero value from the variable declaration")
	}
}

func (a *Pass) checkTimeVariableNaming(name cst.Token, typeNode cst.Node, _ *cst.ExpressionList) {
	typeName := cst.Spelling(typeNode)
	if typeName == "time.Duration" && hasTimeUnitSuffix(name.Src()) {
		a.Report(name, "time.Duration name should not include a unit suffix")
	}
}

func (a *Pass) inspectVarSpecList(names *cst.IdentifierList, typeNode cst.Node, values *cst.ExpressionList, inspect func(*Pass, cst.Token, cst.Node, *cst.ExpressionList)) {
	for _, name := range identifierTokens(names) {
		inspect(a, name, typeNode, values)
	}
}

func (a *Pass) packageDeclaration() bool {
	for _, ancestor := range a.ancestors {
		switch ancestor.(type) {
		case *cst.FunctionDecl, *cst.MethodDecl, *cst.FunctionLit:
			return false
		}
	}
	return true
}

func valueIsError(typeName string, values *cst.ExpressionList) bool {
	if strings.HasPrefix(typeName, "error") {
		return true
	}
	if values == nil {
		return false
	}
	call, ok := values.Expression.(*cst.PrimaryExpr)
	name := callName(call)
	return ok && (name == "errors.New" || name == "fmt.Errorf")
}

func zeroValue(node cst.Node) bool {
	spelling := cst.Spelling(node)
	return spelling == "nil" || spelling == "false" || spelling == "0" || spelling == `""` || spelling == "''"
}
