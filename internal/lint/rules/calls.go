package rules

import (
	"fmt"
	"go/ast"
	"strings"
)

func (a *analyzer) checkCall(call *ast.CallExpr) {
	name := callName(call)
	switch name {
	case "runtime.GC":
		a.report("call-to-gc", call, "avoid explicit garbage collection")
	case "errors.New":
		a.checkErrorMessage(call)
		if len(call.Args) == 1 {
			if inner, ok := call.Args[0].(*ast.CallExpr); ok && callName(inner) == "fmt.Sprintf" {
				a.report(
					"errorf",
					call,
					"replace errors.New(fmt.Sprintf(...)) with fmt.Errorf(...)",
				)
			}
		}
	case "fmt.Errorf":
		a.checkErrorMessage(call)
		if len(call.Args) > 0 && literalWithoutFormatting(call.Args[0]) {
			a.report(
				"use-errors-new",
				call,
				"replace fmt.Errorf with errors.New for a static message",
			)
		}
	case "fmt.Sprintf", "fmt.Fprintf", "fmt.Printf":
		if len(call.Args) > 0 && literalWithoutFormatting(call.Args[0]) {
			a.report("unnecessary-format", call, "formatting call has no formatting directive")
		}
	case "print", "println":
		a.report("use-fmt-print", call, "use fmt.Print or fmt.Println instead of the builtin")
	case "sort.Slice", "sort.SliceStable":
		a.report("use-slices-sort", call, "use slices.Sort or slices.SortFunc when possible")
	case "context.WithValue":
		if len(call.Args) >= 2 && basicContextKey(call.Args[1]) {
			a.report(
				"context-keys-type",
				call.Args[1],
				"context key should use a dedicated, unexported type",
			)
		}
	case "time.Date":
		a.checkTimeDate(call)
	}
	if isDeepExit(name) && !a.insideMainOrInit() {
		a.report("deep-exit", call, "process-exit calls should be confined to main or init")
	}
	if name == "string" && len(call.Args) == 1 && integerLooking(call.Args[0]) {
		a.report(
			"string-of-int",
			call,
			"integer-to-string conversion yields one rune; use string(rune(value)) or strconv.Itoa",
		)
	}
	if isErrorConstructor(name) {
		a.checkErrorMessage(call)
	}
	if a.expressionStatement(call) && likelyReturnsError(name) {
		a.report("unhandled-error", call, fmt.Sprintf("error returned by %s is ignored", name))
	}
	if a.insideWaitGroupGo() &&
		(name == "panic" || name == "recover" || strings.HasSuffix(name, ".Done")) {
		a.report(
			"forbidden-call-in-wg-go",
			call,
			fmt.Sprintf("%s must not be called inside WaitGroup.Go", name),
		)
	}
}
