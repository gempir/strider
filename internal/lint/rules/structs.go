package rules

import (
	"fmt"
	"go/ast"
	"reflect"
	"strconv"
	"strings"
)

func (a *analyzer) checkStruct(structure *ast.StructType) {
	if _, ok := a.parent().(*ast.TypeSpec); !ok {
		a.report(
			"nested-structs",
			structure,
			"move nested anonymous struct types to named declarations",
		)
	}
	for _, field := range structure.Fields.List {
		for _, name := range field.Names {
			a.checkIdentifierName(name)
		}
		if field.Tag != nil {
			a.checkStructTag(field.Tag)
		}
	}
}

func (a *analyzer) checkStructTag(literal *ast.BasicLit) {
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		a.report("struct-tag", literal, "struct tag is not a valid quoted string")
		return
	}
	tag := reflect.StructTag(value)
	for _, key := range []string{"json", "xml", "yaml", "toml", "form", "validate"} {
		if raw, ok := tag.Lookup(key); ok {
			name := strings.Split(raw, ",")[0]
			if strings.ContainsAny(name, " \t\n\r\"") {
				a.report(
					"struct-tag",
					literal,
					fmt.Sprintf("%s tag contains invalid whitespace or quoting", key),
				)
			}
		}
	}
	if value != "" && !validStructTagSyntax(value) {
		a.report("struct-tag", literal, "struct tag has invalid key:value syntax")
	}
}

func validStructTagSyntax(value string) bool {
	for value != "" {
		value = strings.TrimLeft(value, " ")
		colon := strings.IndexByte(value, ':')
		if colon <= 0 {
			return false
		}
		for _, r := range value[:colon] {
			if r <= ' ' || r == ':' || r == '"' || r == '\\' {
				return false
			}
		}
		value = value[colon+1:]
		if len(value) == 0 || value[0] != '"' {
			return false
		}
		_, rest, ok := consumeQuoted(value)
		if !ok {
			return false
		}
		value = rest
	}
	return true
}

func consumeQuoted(value string) (string, string, bool) {
	for index := 1; index < len(value); index++ {
		if value[index] == '\\' {
			index++
			continue
		}
		if value[index] == '"' {
			return value[:index+1], value[index+1:], true
		}
	}
	return "", value, false
}
