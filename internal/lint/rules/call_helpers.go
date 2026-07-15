package rules

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"unicode"
)

func (a *analyzer) checkErrorMessage(call *ast.CallExpr) {
	if len(call.Args) == 0 {
		return
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil || value == "" {
		return
	}
	first, _ := utf8Decode(value)
	badEnd := strings.HasSuffix(value, ".") || strings.HasSuffix(value, ":") ||
		strings.HasSuffix(value, "!") ||
		strings.HasSuffix(value, "\n")
	if unicode.IsUpper(first) || badEnd {
		a.report(
			"error-strings",
			literal,
			"error string should not be capitalized or end with punctuation",
		)
	}
}

func utf8Decode(value string) (rune, int) {
	for _, r := range value {
		return r, len(string(r))
	}
	return 0, 0
}

func literalWithoutFormatting(expr ast.Expr) bool {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(literal.Value)
	return err == nil && !strings.Contains(value, "%")
}

func basicContextKey(expr ast.Expr) bool {
	switch n := expr.(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		return n.Name == "true" || n.Name == "false" || n.Name == "nil"
	}
	return false
}

func isDeepExit(name string) bool {
	return name == "os.Exit" || strings.HasPrefix(name, "log.Fatal") ||
		strings.HasPrefix(name, "log.Panic")
}

func isErrorConstructor(name string) bool {
	return name == "errors.New" || name == "fmt.Errorf" || strings.HasSuffix(name, ".Errorf") ||
		strings.HasSuffix(name, ".Wrap") ||
		strings.HasSuffix(name, ".Wrapf")
}

func integerLooking(expr ast.Expr) bool {
	switch n := expr.(type) {
	case *ast.BasicLit:
		return n.Kind == token.INT
	case *ast.CallExpr:
		return callName(n) == "int" || strings.HasPrefix(callName(n), "int") ||
			strings.HasPrefix(callName(n), "uint")
	}
	return false
}

func likelyReturnsError(name string) bool {
	base := name
	if dot := strings.LastIndex(base, "."); dot >= 0 {
		base = base[dot+1:]
	}
	return base == "Close" || base == "Flush" || base == "Write" || base == "Remove" ||
		base == "Rename" ||
		base == "Chdir" ||
		base == "Setenv" ||
		base == "Unmarshal" ||
		base == "Encode" ||
		strings.HasPrefix(base, "Save")
}

func (a *analyzer) expressionStatement(call *ast.CallExpr) bool {
	statement, ok := a.parent().(*ast.ExprStmt)
	return ok && statement.X == call
}

func (a *analyzer) insideMainOrInit() bool {
	for index := len(a.ancestors) - 1; index >= 0; index-- {
		if fn, ok := a.ancestors[index].(*ast.FuncDecl); ok {
			return fn.Name.Name == "main" || fn.Name.Name == "init"
		}
	}
	return false
}

func (a *analyzer) insideWaitGroupGo() bool {
	for index := len(a.ancestors) - 1; index >= 0; index-- {
		call, ok := a.ancestors[index].(*ast.CallExpr)
		if ok && strings.HasSuffix(callName(call), ".Go") && len(call.Args) > 0 {
			if _, ok := call.Args[0].(*ast.FuncLit); ok {
				return true
			}
		}
	}
	return false
}
