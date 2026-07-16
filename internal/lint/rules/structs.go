package rules

import (
	"fmt"
	"go/ast"
	"reflect"
	"sort"
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
	tags, valid := parseStructTagValues(value)
	if !valid {
		a.report("struct-tag", literal, "struct tag has invalid key:value syntax")
		return
	}
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	importsGoFlags := a.importsPath("github.com/jessevdk/go-flags")
	for _, key := range keys {
		if len(tags[key]) > 1 && !(importsGoFlags && goFlagsRepeatedTag(key)) {
			a.report("struct-tag", literal, fmt.Sprintf("duplicate struct tag %q", key))
		}
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
			switch key {
			case "json":
				a.checkJSONTagOptions(literal, raw)
			case "xml":
				a.checkXMLTagOptions(literal, raw)
			}
		}
	}
}

func parseStructTagValues(value string) (map[string][]string, bool) {
	tags := make(map[string][]string)
	for value != "" {
		value = strings.TrimLeft(value, " ")
		if value == "" {
			break
		}
		colon := strings.IndexByte(value, ':')
		if colon <= 0 {
			return nil, false
		}
		for _, r := range value[:colon] {
			if r <= ' ' || r == ':' || r == '"' || r == '\\' {
				return nil, false
			}
		}
		key := value[:colon]
		value = value[colon+1:]
		if len(value) == 0 || value[0] != '"' {
			return nil, false
		}
		quoted, rest, ok := consumeQuoted(value)
		if !ok {
			return nil, false
		}
		decoded, err := strconv.Unquote(quoted)
		if err != nil {
			return nil, false
		}
		tags[key] = append(tags[key], decoded)
		value = rest
	}
	return tags, true
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

func (a *analyzer) importsPath(wanted string) bool {
	for _, path := range a.imports {
		if path == wanted {
			return true
		}
	}
	return false
}

func goFlagsRepeatedTag(key string) bool {
	switch key {
	case "choice", "optional-value", "default":
		return true
	default:
		return false
	}
}

func (a *analyzer) checkJSONTagOptions(literal *ast.BasicLit, tag string) {
	parts := strings.Split(tag, ",")
	seen := make(map[string]bool)
	for index, option := range parts[1:] {
		if option == "" {
			if index == len(parts[1:])-1 {
				a.report("struct-tag", literal, "json tag has an empty trailing option")
			}
			continue
		}
		if seen[option] {
			a.report("struct-tag", literal, fmt.Sprintf("json tag has duplicate option %q", option))
		}
		seen[option] = true
		if !validJSONTagOption(option) {
			a.report("struct-tag", literal, fmt.Sprintf("json tag has unknown option %q", option))
		}
	}
}

func validJSONTagOption(option string) bool {
	switch option {
	case "omitempty", "omitzero", "string", "inline", "unknown", "embed":
		return true
	}
	if value, ok := strings.CutPrefix(option, "case:"); ok {
		return value == "ignore" || value == "strict"
	}
	if value, ok := strings.CutPrefix(option, "format:"); ok {
		return value != ""
	}
	return false
}

func (a *analyzer) checkXMLTagOptions(literal *ast.BasicLit, tag string) {
	parts := strings.Split(tag, ",")
	seen := make(map[string]bool)
	for _, option := range parts[1:] {
		if option == "" {
			continue
		}
		if seen[option] {
			a.report("struct-tag", literal, fmt.Sprintf("xml tag has duplicate option %q", option))
		}
		seen[option] = true
		if !validXMLTagOption(option) {
			a.report("struct-tag", literal, fmt.Sprintf("xml tag has unknown option %q", option))
		}
	}
}

func validXMLTagOption(option string) bool {
	switch option {
	case "attr", "chardata", "cdata", "innerxml", "comment", "omitempty", "any":
		return true
	default:
		return false
	}
}
