package rules

import (
	"fmt"
	"go/token"
	"strconv"
	"strings"
	"unicode"

	"github.com/gempir/strider/internal/cst"
)

func (a *cstAnalyzer) checkConcreteCall(call *cst.PrimaryExpr) {
	if call == nil || !strings.HasPrefix(cst.Kind(call.Postfix), "Arguments") {
		return
	}
	name := cst.Spelling(call.PrimaryExpr)
	arguments := concreteCallArguments(call.Postfix)
	switch name {
	case "runtime.GC":
		a.report("call-to-gc", call, "avoid explicit garbage collection")
	case "errors.New":
		if len(arguments) == 1 {
			if inner, ok := arguments[0].(*cst.PrimaryExpr); ok && concreteCallName(inner) == "fmt.Sprintf" {
				a.report("prefer-fmt-errorf", call, "replace errors.New(fmt.Sprintf(...)) with fmt.Errorf(...)")
			}
		}
	case "fmt.Errorf":
		if len(arguments) > 0 && concreteLiteralWithoutFormatting(arguments[0]) {
			a.report("use-errors-new", call, "replace fmt.Errorf with errors.New for a static message")
		}
	case "fmt.Sprintf", "fmt.Fprintf", "fmt.Printf":
		if len(arguments) > 0 && concreteLiteralWithoutFormatting(arguments[0]) {
			a.report("unnecessary-format", call, "formatting call has no formatting directive")
		}
	case "print", "println":
		a.report("use-fmt-print", call, "use fmt.Print or fmt.Println instead of the builtin")
	case "sort.Slice", "sort.SliceStable":
		a.report("use-slices-sort", call, "use slices.Sort or slices.SortFunc when possible")
	case "time.Date":
		a.checkConcreteTimeDate(arguments)
	}
	if isDeepExit(name) && !a.insideConcreteMainOrInit() {
		a.report("deep-exit", call, "process-exit calls should be confined to main or init")
	}
	if name == "string" && len(arguments) == 1 && concreteIntegerLooking(arguments[0]) {
		a.report("string-of-int", call, "integer-to-string conversion yields one rune; use string(rune(value)) or strconv.Itoa")
	}
	if isErrorConstructor(name) {
		a.checkConcreteErrorMessage(arguments)
	}
}

func concreteIntegerLooking(node cst.Node) bool {
	if literal, ok := node.(*cst.BasicLit); ok {
		return literal.Ch() == token.INT
	}
	call, ok := node.(*cst.PrimaryExpr)
	if !ok {
		return false
	}
	name := concreteCallName(call)
	return name == "int" || strings.HasPrefix(name, "int") || strings.HasPrefix(name, "uint")
}

func (a *cstAnalyzer) checkConcreteTimeDate(arguments []cst.Node) {
	limits := []struct {
		index, minimum, maximum int
		label                   string
	}{{1, 1, 12, "month"}, {2, 1, 31, "day"}, {3, 0, 23, "hour"}, {4, 0, 59, "minute"}, {5, 0, 59, "second"}, {6, 0, 999999999, "nanosecond"}}
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
			a.report("time-date", literal, fmt.Sprintf("time.Date %s argument %d is outside %d..%d", limit.label, value, limit.minimum, limit.maximum))
		}
	}
}

func concreteCallName(call *cst.PrimaryExpr) string {
	if call == nil || !strings.HasPrefix(cst.Kind(call.Postfix), "Arguments") {
		return ""
	}
	return cst.Spelling(call.PrimaryExpr)
}

func concreteCallArguments(arguments cst.Node) []cst.Node {
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
		result = appendConcreteExpressionList(result, current.ExpressionList)
	case *cst.Arguments3:
		if current.TypeNode != nil {
			result = append(result, current.TypeNode)
		}
		result = appendConcreteExpressionList(result, current.ExpressionList)
	}
	return result
}

func appendConcreteExpressionList(result []cst.Node, current *cst.ExpressionList) []cst.Node {
	for ; current != nil; current = current.List {
		if current.Expression != nil {
			result = append(result, current.Expression)
		}
	}
	return result
}

func (a *cstAnalyzer) checkConcreteErrorMessage(arguments []cst.Node) {
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
	first, _ := utf8Decode(value)
	badEnd := strings.HasSuffix(value, ".") || strings.HasSuffix(value, ":") || strings.HasSuffix(value, "!") || strings.HasSuffix(value, "\n")
	if unicode.IsUpper(first) || badEnd {
		a.report("error-strings", literal, "error string should not be capitalized or end with punctuation")
	}
}

func concreteLiteralWithoutFormatting(node cst.Node) bool {
	literal, ok := node.(*cst.BasicLit)
	if !ok || literal.Ch() != token.STRING {
		return false
	}
	value, err := strconv.Unquote(literal.Src())
	return err == nil && !strings.Contains(value, "%")
}

func (a *cstAnalyzer) insideConcreteMainOrInit() bool {
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
