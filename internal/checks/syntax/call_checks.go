//strider:ignore-file cognitive-complexity,cyclomatic-complexity,modifies-parameter
package syntax

import (
	"fmt"
	"go/token"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gempir/strider/internal/cst"
)

type callFacts struct {
	node      *cst.PrimaryExpr
	name      string
	arguments []cst.Node
}

func (a *Pass) callFacts(call *cst.PrimaryExpr) callFacts {
	if facts, ok := a.calls[call]; ok {
		return facts
	}
	facts := callFacts{
		node: call,
	}
	if call == nil || !cst.IsArguments(call.Postfix) {
		return facts
	}
	facts.name = cst.Spelling(call.PrimaryExpr)
	facts.arguments = callArguments(call.Postfix)
	if a.calls == nil {
		a.calls = make(map[*cst.PrimaryExpr]callFacts)
	}
	a.calls[call] = facts
	return facts
}

func (a *Pass) checkCallToGC(call callFacts) {
	if call.name == "runtime.GC" {
		a.Report(call.node, "avoid explicit garbage collection")
	}
}

func (a *Pass) checkPreferFmtErrorf(call callFacts) {
	if call.name != "errors.New" || len(call.arguments) != 1 {
		return
	}
	if inner, ok := call.arguments[0].(*cst.PrimaryExpr); ok && callName(inner) == "fmt.Sprintf" {
		a.Report(call.node, "replace errors.New(fmt.Sprintf(...)) with fmt.Errorf(...)")
	}
}

func (a *Pass) checkUseErrorsNew(call callFacts) {
	if call.name == "fmt.Errorf" && len(call.arguments) > 0 && literalWithoutFormatting(call.arguments[0]) {
		a.Report(call.node, "replace fmt.Errorf with errors.New for a static message")
	}
}

func (a *Pass) checkUnnecessaryFormat(call callFacts) {
	switch call.name {
	case "fmt.Sprintf", "fmt.Fprintf", "fmt.Printf":
		if len(call.arguments) > 0 && literalWithoutFormatting(call.arguments[0]) {
			a.Report(call.node, "formatting call has no formatting directive")
		}
	}
}

func (a *Pass) checkUseFmtPrint(call callFacts) {
	if call.name == "print" || call.name == "println" {
		a.Report(call.node, "use fmt.Print or fmt.Println instead of the builtin")
	}
}

func (a *Pass) checkUseSlicesSort(call callFacts) {
	if call.name == "sort.Slice" || call.name == "sort.SliceStable" {
		a.Report(call.node, "use slices.Sort or slices.SortFunc when possible")
	}
}

func (a *Pass) checkTimeDateCall(call callFacts) {
	if call.name == "time.Date" {
		a.checkTimeDate(call.arguments)
	}
}

func (a *Pass) checkDeepExit(call callFacts) {
	if isDeepExit(call.name) && !a.insideMainOrInit() {
		a.Report(call.node, "process-exit calls should be confined to main or init")
	}
}

func (a *Pass) checkStringOfInt(call callFacts) {
	if call.name == "string" && len(call.arguments) == 1 && integerLooking(call.arguments[0]) {
		a.Report(call.node, "integer-to-string conversion yields one rune; use string(rune(value)) or strconv.Itoa")
	}
}

func (a *Pass) checkErrorStringCall(call callFacts) {
	if isErrorConstructor(call.name) {
		a.checkErrorMessage(call.arguments)
	}
}

func integerLooking(node cst.Node) bool {
	if literal, ok := node.(*cst.BasicLit); ok {
		return literal.Ch() == token.INT
	}
	call, ok := node.(*cst.PrimaryExpr)
	if !ok {
		return false
	}
	name := callName(call)
	return name == "int" || strings.HasPrefix(name, "int") || strings.HasPrefix(name, "uint")
}

func (a *Pass) checkTimeDate(arguments []cst.Node) {
	limits := []struct {
		index, minimum, maximum int
		label                   string
	}{
		{
			1,
			1,
			12,
			"month",
		},
		{
			2,
			1,
			31,
			"day",
		},
		{
			3,
			0,
			23,
			"hour",
		},
		{
			4,
			0,
			59,
			"minute",
		},
		{
			5,
			0,
			59,
			"second",
		},
		{
			6,
			0,
			999999999,
			"nanosecond",
		},
	}
	for _, limit := range limits {
		if limit.index >= len(arguments) {
			continue
		}
		literal, ok := arguments[limit.index].(*cst.BasicLit)
		if !ok || literal.Ch() != token.INT {
			continue
		}
		value, err := strconv.Atoi(literal.Src())
		if err == nil && (value < limit.minimum || value > limit.maximum) {
			a.Report(literal, fmt.Sprintf("time.Date %s argument %d is outside %d..%d", limit.label, value, limit.minimum, limit.maximum))
		}
	}
}

func callName(call *cst.PrimaryExpr) string {
	if call == nil || !cst.IsArguments(call.Postfix) {
		return ""
	}
	return cst.Spelling(call.PrimaryExpr)
}

func callArguments(arguments cst.Node) []cst.Node {
	result := []cst.Node{}
	switch current := arguments.(type) {
	case *cst.Arguments:
		if current.Expression != nil {
			result = append(result, current.Expression)
		}
	case *cst.Arguments1:
		if current.Expression != nil {
			result = append(result, current.Expression)
		} else if current.TypeNode != nil {
			result = append(result, current.TypeNode)
		}
	case *cst.Arguments2:
		result = appendExpressionList(result, current.ExpressionList)
	case *cst.Arguments3:
		if current.TypeNode != nil {
			result = append(result, current.TypeNode)
		}
		result = appendExpressionList(result, current.ExpressionList)
	}
	return result
}

func appendExpressionList(result []cst.Node, current *cst.ExpressionList) []cst.Node {
	for ; current != nil; current = current.List {
		if current.Expression != nil {
			result = append(result, current.Expression)
		}
	}
	return result
}

func (a *Pass) checkErrorMessage(arguments []cst.Node) {
	if len(arguments) == 0 {
		return
	}
	literal, ok := arguments[0].(*cst.BasicLit)
	if !ok || literal.Ch() != token.STRING {
		return
	}
	value, err := strconv.Unquote(literal.Src())
	if err != nil || value == "" {
		return
	}
	first, _ := utf8.DecodeRuneInString(value)
	badEnd := strings.HasSuffix(value, ".") || strings.HasSuffix(value, ":") || strings.HasSuffix(value, "!") || strings.HasSuffix(value, "\n")
	if unicode.IsUpper(first) || badEnd {
		a.Report(literal, "error string should not be capitalized or end with punctuation")
	}
}

func literalWithoutFormatting(node cst.Node) bool {
	literal, ok := node.(*cst.BasicLit)
	if !ok || literal.Ch() != token.STRING {
		return false
	}
	value, err := strconv.Unquote(literal.Src())
	return err == nil && !strings.Contains(value, "%")
}

func (a *Pass) insideMainOrInit() bool {
	for index := len(a.ancestors) - 1; index >= 0; index-- {
		function, ok := a.ancestors[index].(*cst.FunctionDecl)
		if !ok || function.FunctionName == nil {
			continue
		}
		name := function.FunctionName.IDENT.Src()
		return name == "main" || name == "init"
	}
	return false
}
