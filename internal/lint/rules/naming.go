package rules

import (
	"fmt"
	"go/ast"
	"strings"
	"unicode"
)

func (a *analyzer) checkIdentifierName(name *ast.Ident) {
	if name == nil || name.Name == "_" {
		return
	}
	if strings.HasPrefix(name.Name, "_") {
		a.report(
			"unexported-naming",
			name,
			"unexported identifier should not begin with an underscore",
		)
	}
	if strings.Contains(name.Name, "_") && !strings.HasPrefix(name.Name, "Test") &&
		!strings.HasPrefix(name.Name, "Benchmark") &&
		!strings.HasPrefix(name.Name, "Example") {
		a.report("var-naming", name, "identifier should use MixedCaps rather than underscores")
	}
	if builtinIdentifiers[name.Name] {
		a.report(
			"redefines-builtin-id",
			name,
			fmt.Sprintf("identifier %s shadows a predeclared identifier", name.Name),
		)
	}
	if a.importNames[name.Name] {
		a.report(
			"import-shadowing",
			name,
			fmt.Sprintf("identifier %s shadows an imported package", name.Name),
		)
	}
}

var builtinIdentifiers = map[string]bool{
	"any":        true,
	"append":     true,
	"bool":       true,
	"byte":       true,
	"cap":        true,
	"clear":      true,
	"close":      true,
	"comparable": true,
	"complex":    true,
	"complex128": true,
	"complex64":  true,
	"copy":       true,
	"delete":     true,
	"error":      true,
	"false":      true,
	"float32":    true,
	"float64":    true,
	"imag":       true,
	"int":        true,
	"int16":      true,
	"int32":      true,
	"int64":      true,
	"int8":       true,
	"iota":       true,
	"len":        true,
	"make":       true,
	"max":        true,
	"min":        true,
	"new":        true,
	"nil":        true,
	"panic":      true,
	"print":      true,
	"println":    true,
	"real":       true,
	"recover":    true,
	"rune":       true,
	"string":     true,
	"true":       true,
	"uint":       true,
	"uint16":     true,
	"uint32":     true,
	"uint64":     true,
	"uint8":      true,
	"uintptr":    true,
}

func identifierName(value string) bool {
	for index, r := range value {
		if index == 0 && !unicode.IsLetter(r) && r != '_' {
			return false
		}
		if index > 0 && !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return value != ""
}

func builtinType(value string) bool {
	return builtinIdentifiers[value]
}

func hasTimeUnitSuffix(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range []string{
		"ns",
		"us",
		"ms",
		"sec",
		"secs",
		"second",
		"seconds",
		"min",
		"mins",
		"minute",
		"minutes",
		"hour",
		"hours",
	} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}
