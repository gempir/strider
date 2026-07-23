package syntax

import (
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var cgoPointerIdentifier = regexp.MustCompile(`^_C(func|var)_.+$`)

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

func utf8Decode(value string) (rune, int) {
	return utf8.DecodeRuneInString(value)
}

func isDeepExit(name string) bool {
	return name == "os.Exit" || strings.HasPrefix(name, "log.Fatal") || strings.HasPrefix(name, "log.Panic")
}

func isErrorConstructor(name string) bool {
	return name == "errors.New" || name == "fmt.Errorf" || strings.HasSuffix(name, ".Errorf") || strings.HasSuffix(name, ".Wrap") || strings.HasSuffix(name, ".Wrapf")
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

func identifierName(value string) bool {
	for index, current := range value {
		if index == 0 && !unicode.IsLetter(current) && current != '_' {
			return false
		}
		if index > 0 && !unicode.IsLetter(current) && !unicode.IsDigit(current) && current != '_' {
			return false
		}
	}
	return value != ""
}

func builtinType(value string) bool {
	return builtinIdentifiers[value]
}

func marshalMethod(name string) bool {
	return strings.HasPrefix(name, "Marshal") || strings.HasPrefix(name, "Unmarshal") || strings.HasPrefix(name, "Encode") || strings.HasPrefix(name, "Decode")
}

func pathContains(path, element string) bool {
	for _, part := range strings.Split(filepath.Clean(path), string(filepath.Separator)) {
		if part == element {
			return true
		}
	}
	return false
}

func bidirectionalControl(character rune) bool {
	switch character {
	case '\u202a', '\u202b', '\u202c', '\u202d', '\u202e', '\u2066', '\u2067', '\u2068', '\u2069':
		return true
	default:
		return false
	}
}

func legacyBuildTerms(text string) []string {
	terms := strings.Fields(strings.TrimSpace(strings.TrimPrefix(text, "// +build")))
	slices.Sort(terms)
	return terms
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
		for _, current := range value[:colon] {
			if current <= ' ' || current == ':' || current == '"' || current == '\\' {
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

func validXMLTagOption(option string) bool {
	switch option {
	case "attr", "chardata", "cdata", "innerxml", "comment", "omitempty", "any":
		return true
	default:
		return false
	}
}
