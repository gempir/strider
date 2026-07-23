package semantic

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gempir/strider/internal/diagnostic"
)

type invalidPrintfCallCheck struct{}

type printfUse struct {
	raw   string
	verb  rune
	value int
	stars []int
}

func (invalidPrintfCallCheck) Meta() Meta {
	return Meta{
		Code:            "invalid-printf-call",
		Summary:         "detect malformed printf formats and mismatched arguments",
		Explanation:     "Printf-style calls interpret a small language of verbs, argument indexes, widths, and precisions. A malformed format, missing or extra argument, non-integer star argument, unsupported wrapping verb, or incompatible value type produces broken output at runtime.",
		GoodExample:     `fmt.Printf("%d %s", count, name)`,
		BadExample:      `fmt.Printf("%d %s", name, count)`,
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (invalidPrintfCallCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
		},
		func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || call.Ellipsis.IsValid() {
				return true
			}
			formatIndex, ok := dynamicPrintfFormatIndex(pass, call)
			if !ok || formatIndex >= len(call.Args) {
				return true
			}
			formatValue := pass.TypesInfo.Types[call.Args[formatIndex]].Value
			if formatValue == nil || formatValue.Kind() != constant.String {
				return true
			}
			formatText := constant.StringVal(formatValue)
			uses, indexed, err := parsePrintfUses(formatText)
			if err != nil {
				pass.Report(call.Args[formatIndex], err.Error())
				return true
			}
			arguments := call.Args[formatIndex+1:]
			name := printfCallName(pass, call)
			if message := validatePrintfUses(pass, name, uses, arguments, indexed); message != "" {
				pass.Report(call.Args[formatIndex], message)
			}
			return true
		},
	)
}

func printfCallName(pass *Pass, call *ast.CallExpr) string {
	function := calledFunction(pass.TypesInfo, call.Fun)
	if function == nil || function.Pkg() == nil {
		return "printf-style call"
	}
	return function.Pkg().Name() + "." + function.Name()
}

func parsePrintfUses(formatText string) ([]printfUse, bool, error) {
	uses := make([]printfUse, 0)
	nextArgument := 0
	anyIndex := false
	for offset := 0; offset < len(formatText); {
		if formatText[offset] != '%' {
			_, width := utf8.DecodeRuneInString(formatText[offset:])
			offset += width
			continue
		}
		start := offset
		offset++
		for offset < len(formatText) && strings.ContainsRune("#0+- ", rune(formatText[offset])) {
			offset++
		}

		pendingIndex := -1
		if index, end, present, err := printfIndex(formatText, offset); err != nil {
			return nil, false, err
		} else if present {
			anyIndex = true
			pendingIndex, offset = index, end
		}

		stars := make([]int, 0, 2)
		if offset < len(formatText) && formatText[offset] == '*' {
			star := pendingIndex
			if star < 0 {
				star = nextArgument
			}
			stars = append(stars, star)
			nextArgument = star + 1
			pendingIndex = -1
			offset++
		} else {
			for offset < len(formatText) && formatText[offset] >= '0' && formatText[offset] <= '9' {
				offset++
			}
		}

		if offset < len(formatText) && formatText[offset] == '.' {
			offset++
			if index, end, present, err := printfIndex(formatText, offset); err != nil {
				return nil, false, err
			} else if present {
				anyIndex = true
				pendingIndex, offset = index, end
			}
			if offset < len(formatText) && formatText[offset] == '*' {
				star := pendingIndex
				if star < 0 {
					star = nextArgument
				}
				stars = append(stars, star)
				nextArgument = star + 1
				pendingIndex = -1
				offset++
			} else {
				for offset < len(formatText) && formatText[offset] >= '0' && formatText[offset] <= '9' {
					offset++
				}
			}
		}

		if pendingIndex < 0 {
			if index, end, present, err := printfIndex(formatText, offset); err != nil {
				return nil, false, err
			} else if present {
				anyIndex = true
				pendingIndex, offset = index, end
			}
		}
		if offset >= len(formatText) {
			return nil, false, fmt.Errorf("printf format %q is missing a verb", formatText[start:])
		}
		verb, width := utf8.DecodeRuneInString(formatText[offset:])
		offset += width
		valueIndex := pendingIndex
		if verb == '%' {
			valueIndex = -1
		} else {
			if valueIndex < 0 {
				valueIndex = nextArgument
			}
			nextArgument = valueIndex + 1
		}
		uses = append(uses, printfUse{
			raw:   formatText[start:offset],
			verb:  verb,
			value: valueIndex,
			stars: stars,
		})
	}
	return uses, anyIndex, nil
}

func printfIndex(formatText string, offset int) (int, int, bool, error) {
	if offset >= len(formatText) || formatText[offset] != '[' {
		return 0, offset, false, nil
	}
	end := strings.IndexByte(formatText[offset:], ']')
	if end < 0 {
		return 0, offset, false, fmt.Errorf("printf format has an argument index without a closing bracket")
	}
	end += offset
	number := formatText[offset+1 : end]
	index, err := strconv.Atoi(number)
	if err != nil || index <= 0 {
		return 0, offset, false, fmt.Errorf("printf format has invalid argument index [%s]", number)
	}
	return index - 1, end + 1, true, nil
}

func validatePrintfUses(pass *Pass, callName string, uses []printfUse, arguments []ast.Expr, indexed bool) string {
	highest := -1
	for _, use := range uses {
		customFormatter := use.value >= 0 && use.value < len(arguments) && hasFormatMethod(pass.TypesInfo.TypeOf(arguments[use.value]))
		if !knownPrintfVerb(use.verb) && !customFormatter {
			return fmt.Sprintf("%s format %s has unknown verb %c", callName, use.raw, use.verb)
		}
		if use.verb == 'w' && callName != "fmt.Errorf" {
			return fmt.Sprintf("%s does not support the %%w error-wrapping verb", callName)
		}
		for _, index := range use.stars {
			if index >= len(arguments) {
				return fmt.Sprintf("%s format %s reads argument %d, but call has %d values", callName, use.raw, index+1, len(arguments))
			}
			if index > highest {
				highest = index
			}
			if !printfIntegerType(pass.TypesInfo.TypeOf(arguments[index])) {
				return fmt.Sprintf("%s format %s uses a non-integer argument for *", callName, use.raw)
			}
		}
		if use.value < 0 {
			continue
		}
		if use.value >= len(arguments) {
			return fmt.Sprintf("%s format %s reads argument %d, but call has %d values", callName, use.raw, use.value+1, len(arguments))
		}
		if use.value > highest {
			highest = use.value
		}
		valueType := pass.TypesInfo.TypeOf(arguments[use.value])
		if !printfVerbAccepts(use.verb, valueType) {
			return fmt.Sprintf("%s format %s has argument %d of incompatible type %s", callName, use.raw, use.value+1, types.TypeString(valueType, nil))
		}
	}
	if !indexed && highest+1 < len(arguments) {
		return fmt.Sprintf("%s call needs %d values but has %d", callName, highest+1, len(arguments))
	}
	return ""
}

func knownPrintfVerb(verb rune) bool {
	return strings.ContainsRune("%bcdeEfFgGopqstTUvwxX", verb)
}

func printfVerbAccepts(verb rune, valueType types.Type) bool {
	return printfVerbAcceptsRecursive(verb, valueType, true, make(map[types.Type]bool))
}

func printfVerbAcceptsRecursive(verb rune, valueType types.Type, top bool, seen map[types.Type]bool) bool {
	if valueType == nil {
		return true
	}
	if _, ok := valueType.Underlying().(*types.Interface); ok || hasFormatMethod(valueType) {
		return true
	}
	if seen[valueType] {
		return true
	}
	seen[valueType] = true
	defer delete(seen, valueType)

	if verb == 'w' {
		return implementsError(valueType)
	}
	if verb == 'v' || verb == 'T' {
		return true
	}
	if (verb == 's' || verb == 'q' || verb == 'x' || verb == 'X') && (hasStringMethod(valueType) || implementsError(valueType)) {
		return true
	}

	underlying := valueType.Underlying()
	if pointer, ok := underlying.(*types.Pointer); ok {
		if verb == 'p' || strings.ContainsRune("bdoOxX", verb) {
			return true
		}
		if top {
			return printfVerbAcceptsRecursive(verb, pointer.Elem(), false, seen)
		}
		return false
	}
	switch aggregate := underlying.(type) {
	case *types.Slice:
		if verb == 'p' || ((verb == 's' || verb == 'q' || verb == 'x' || verb == 'X') && isByteType(aggregate.Elem())) {
			return true
		}
		return printfVerbAcceptsRecursive(verb, aggregate.Elem(), false, seen)
	case *types.Array:
		if (verb == 's' || verb == 'q' || verb == 'x' || verb == 'X') && isByteType(aggregate.Elem()) {
			return true
		}
		return printfVerbAcceptsRecursive(verb, aggregate.Elem(), false, seen)
	case *types.Map:
		if verb == 'p' {
			return true
		}
		return printfVerbAcceptsRecursive(verb, aggregate.Key(), false, seen) && printfVerbAcceptsRecursive(verb, aggregate.Elem(), false, seen)
	case *types.Struct:
		for index := range aggregate.NumFields() {
			if !printfVerbAcceptsRecursive(verb, aggregate.Field(index).Type(), false, seen) {
				return false
			}
		}
		return true
	}
	switch verb {
	case 'v', 'T':
		return true
	case 't':
		return printfBasicInfo(valueType, types.IsBoolean)
	case 'c', 'd', 'o', 'O', 'U':
		return printfBasicInfo(valueType, types.IsInteger) || printfPointerLike(valueType)
	case 'e', 'E', 'f', 'F', 'g', 'G':
		return printfBasicInfo(valueType, types.IsFloat|types.IsComplex)
	case 'b':
		return printfBasicInfo(valueType, types.IsInteger|types.IsFloat|types.IsComplex) || printfPointerLike(valueType)
	case 's':
		return printfStringType(valueType)
	case 'q':
		return printfStringType(valueType) || printfBasicInfo(valueType, types.IsInteger)
	case 'x', 'X':
		return printfStringType(valueType) || printfBasicInfo(valueType, types.IsInteger|types.IsFloat|types.IsComplex) || printfPointerLike(valueType)
	case 'p':
		return printfPointerLike(valueType)
	default:
		return verb == '%' || hasFormatMethod(valueType)
	}
}

func isByteType(valueType types.Type) bool {
	basic, ok := valueType.Underlying().(*types.Basic)
	return ok && basic.Kind() == types.Byte
}

func printfBasicInfo(valueType types.Type, wanted types.BasicInfo) bool {
	basic, ok := valueType.Underlying().(*types.Basic)
	return ok && basic.Info()&wanted != 0
}

func printfIntegerType(valueType types.Type) bool {
	return printfBasicInfo(valueType, types.IsInteger)
}

func printfStringType(valueType types.Type) bool {
	if printfBasicInfo(valueType, types.IsString) || hasStringMethod(valueType) || implementsError(valueType) {
		return true
	}
	switch underlying := valueType.Underlying().(type) {
	case *types.Slice:
		return printfBasicInfo(underlying.Elem(), types.IsInteger) && underlying.Elem().Underlying().(*types.Basic).Kind() == types.Byte
	case *types.Array:
		return printfBasicInfo(underlying.Elem(), types.IsInteger) && underlying.Elem().Underlying().(*types.Basic).Kind() == types.Byte
	default:
		return false
	}
}

func printfPointerLike(valueType types.Type) bool {
	switch valueType.Underlying().(type) {
	case *types.Pointer, *types.Chan, *types.Map, *types.Signature, *types.Slice:
		return true
	case *types.Basic:
		basic := valueType.Underlying().(*types.Basic)
		return basic.Kind() == types.UnsafePointer
	default:
		return false
	}
}

func hasFormatMethod(valueType types.Type) bool {
	method, _, _ := types.LookupFieldOrMethod(valueType, true, nil, "Format")
	function, ok := method.(*types.Func)
	if !ok {
		return false
	}
	signature, _ := function.Type().(*types.Signature)
	return signature != nil && signature.Params().Len() == 2 && signature.Results().Len() == 0
}

func hasStringMethod(valueType types.Type) bool {
	method, _, _ := types.LookupFieldOrMethod(valueType, true, nil, "String")
	function, ok := method.(*types.Func)
	if !ok {
		return false
	}
	signature, _ := function.Type().(*types.Signature)
	return signature != nil && signature.Params().Len() == 0 && signature.Results().Len() == 1 && printfBasicInfo(signature.Results().At(0).Type(), types.IsString)
}

func implementsError(valueType types.Type) bool {
	method, _, _ := types.LookupFieldOrMethod(valueType, true, nil, "Error")
	function, ok := method.(*types.Func)
	if !ok {
		return false
	}
	signature, _ := function.Type().(*types.Signature)
	return signature != nil && signature.Params().Len() == 0 && signature.Results().Len() == 1 && printfBasicInfo(signature.Results().At(0).Type(), types.IsString)
}
