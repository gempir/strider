package rules

import (
	"fmt"
	"go/token"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

func (a *Pass) checkStruct(structure *cst.StructType) {
	if len(a.ancestors) == 0 {
		return
	}
	if _, named := a.ancestors[len(a.ancestors)-1].(*cst.TypeDef); !named {
		a.report("nested-structs", structure, "move nested anonymous struct types to named declarations")
	}
}

func (a *Pass) checkStructField(field *cst.FieldDecl) {
	if field.Tag != nil {
		a.checkStructTag(field.Tag.STRING)
	}
}

func (a *Pass) checkStringLiteral(literal *cst.BasicLit) {
	if literal.Ch() != token.STRING {
		return
	}
	value, err := strconv.Unquote(literal.Src())
	if err != nil {
		return
	}
	lower := strings.ToLower(value)
	for _, scheme := range []string{
		"http://",
		"ws://",
		"ftp://",
	} {
		if strings.HasPrefix(lower, scheme) {
			a.report("insecure-url-scheme", literal, fmt.Sprintf("URL uses insecure %s scheme", strings.TrimSuffix(scheme, "://")))
			return
		}
	}
}

func (a *Pass) checkStructTag(literal cst.Token) {
	value, err := strconv.Unquote(literal.Src())
	if err != nil {
		a.report("invalid-struct-tag", literal, "struct tag is not a valid quoted string")
		return
	}
	tags, valid := parseStructTagValues(value)
	if !valid {
		a.report("invalid-struct-tag", literal, "struct tag has invalid key:value syntax")
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
			a.report("invalid-struct-tag", literal, fmt.Sprintf("duplicate struct tag %q", key))
		}
	}
	tag := reflect.StructTag(value)
	for _, key := range []string{
		"json",
		"xml",
		"yaml",
		"toml",
		"form",
		"validate",
	} {
		raw, ok := tag.Lookup(key)
		if !ok {
			continue
		}
		name := strings.Split(raw, ",")[0]
		if strings.ContainsAny(name, " \t\n\r\"") {
			a.report("invalid-struct-tag", literal, fmt.Sprintf("%s tag contains invalid whitespace or quoting", key))
		}
		switch key {
		case "json":
			a.checkJSONTagOptions(literal, raw)
		case "xml":
			a.checkXMLTagOptions(literal, raw)
		}
	}
}

func (a *Pass) importsPath(wanted string) bool {
	return a.imports().paths[wanted]
}

func (a *Pass) checkJSONTagOptions(literal cst.Token, tag string) {
	parts := strings.Split(tag, ",")
	seen := make(map[string]bool)
	for index, option := range parts[1:] {
		if option == "" {
			if index == len(parts[1:])-1 {
				a.report("invalid-struct-tag", literal, "json tag has an empty trailing option")
			}
			continue
		}
		if seen[option] {
			a.report("invalid-struct-tag", literal, fmt.Sprintf("json tag has duplicate option %q", option))
		}
		seen[option] = true
		if !validJSONTagOption(option) {
			a.report("invalid-struct-tag", literal, fmt.Sprintf("json tag has unknown option %q", option))
		}
	}
}

func (a *Pass) checkXMLTagOptions(literal cst.Token, tag string) {
	parts := strings.Split(tag, ",")
	seen := make(map[string]bool)
	for _, option := range parts[1:] {
		if option == "" {
			continue
		}
		if seen[option] {
			a.report("invalid-struct-tag", literal, fmt.Sprintf("xml tag has duplicate option %q", option))
		}
		seen[option] = true
		if !validXMLTagOption(option) {
			a.report("invalid-struct-tag", literal, fmt.Sprintf("xml tag has unknown option %q", option))
		}
	}
}
