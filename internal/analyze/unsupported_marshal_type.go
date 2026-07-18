package analyze

import (
	"fmt"
	"go/types"
	"reflect"
	"strings"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type unsupportedMarshalTypeRule struct {}

type marshalCall struct {
	format string
	ssaArgument int
	sourceArgument int
}

func (unsupportedMarshalTypeRule) Meta() Meta {
	return Meta{
		Code: "unsupported-marshal-type",
		Summary: "detect channels and functions passed to JSON or XML marshaling",
		Explanation: "The standard JSON and XML encoders cannot marshal channel or function values. The restriction also applies when an unsupported value is reachable through an exported, non-ignored field.",
		GoodExample: "json.Marshal(struct{ Name string }{Name: name})",
		BadExample: "json.Marshal(make(chan int))",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (unsupportedMarshalTypeRule) Run(pass *Pass) {
	calls := pass.argumentsByCallPosition()
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok {
					continue
				}
				descriptor, ok := marshalDescriptor(call)
				if !ok || len(call.Common().Args) <= descriptor.ssaArgument {
					continue
				}
				value := unwrapSSAValue(call.Common().Args[descriptor.ssaArgument])
				unsupported, path := unsupportedMarshalType(
					value.Type(),
					descriptor.format,
					make(map[types.Type]bool),
					"",
				)
				if unsupported == nil {
					continue
				}
				node := explicitCallArgument(
					calls[call.Pos()],
					descriptor.sourceArgument,
					call.Pos(),
				)
				message := fmt.Sprintf(
					"%s cannot marshal values of type %s",
					descriptor.format,
					types.TypeString(unsupported, types.RelativeTo(pass.Types)),
				)
				if path != "" {
					message += " via " + path
				}
				pass.Report(node, message)
			}
		}
	}
}

func marshalDescriptor(call ssa.CallInstruction) (marshalCall, bool) {
	switch {
	case isStaticFunction(call, "encoding/json", "Marshal"), isStaticFunction(
			call,
			"encoding/json",
			"MarshalIndent",
		):
		return marshalCall{format:
		"JSON", ssaArgument:
		0, sourceArgument:
		0}, true
	case isStaticFunction(call, "encoding/xml", "Marshal"), isStaticFunction(
			call,
			"encoding/xml",
			"MarshalIndent",
		):
		return marshalCall{format:
		"XML", ssaArgument:
		0, sourceArgument:
		0}, true
	}
	callee := call.Common().StaticCallee()
	if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil || callee.Object().Name() != "Encode" {
		return marshalCall{}, false
	}
	signature, _ := callee.Object().Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		return marshalCall{}, false
	}
	path := callee.Object().Pkg().Path()
	switch path {
	case "encoding/json":
		return marshalCall{format:
		"JSON", ssaArgument:
		1, sourceArgument:
		0}, true
	case "encoding/xml":
		return marshalCall{format:
		"XML", ssaArgument:
		1, sourceArgument:
		0}, true
	default:
		return marshalCall{}, false
	}
}

func unsupportedMarshalType(
	valueType types.Type,
	format string,
	seen map[types.Type]bool,
	path string,
) (types.Type, string) {
	valueType = types.Unalias(valueType)
	if seen[valueType] {
		return nil, ""
	}
	seen[valueType] = true
	defer delete(seen, valueType)
	if named, ok := valueType.(*types.Named); ok {
		if hasCustomMarshaler(named, format) {
			return nil, ""
		}
		return unsupportedMarshalType(named.Underlying(), format, seen, path)
	}
	switch underlying := valueType.Underlying().(type) {
	case *types.Chan, *types.Signature:
		return valueType, path
	case *types.Basic:
		if format == "JSON" && (underlying.Kind() == types.Complex64 || underlying.Kind() == types.Complex128) {
			return valueType, path
		}
		return nil, ""
	case *types.Pointer:
		return unsupportedMarshalType(underlying.Elem(), format, seen, path)
	case *types.Array, *types.Slice:
		var element types.Type
		if array, ok := underlying.(*types.Array); ok {
			element = array.Elem()
		} else {
			element = underlying.(*types.Slice).Elem()
		}
		return unsupportedMarshalType(element, format, seen, path + "[]")
	case *types.Map:
		return unsupportedMarshalType(underlying.Elem(), format, seen, path + "[value]")
	case *types.Struct:
		for index := range underlying.NumFields() {
			field := underlying.Field(index)
			if !field.Exported() || ignoredMarshalField(underlying.Tag(index), format) {
				continue
			}
			fieldPath := field.Name()
			if path != "" {
				fieldPath = path + "." + fieldPath
			}
			if unsupported, nestedPath := unsupportedMarshalType(
				field.Type(),
				format,
				seen,
				fieldPath,
			); unsupported != nil {
				return unsupported, nestedPath
			}
		}
	}
	return nil, ""
}

func hasCustomMarshaler(named *types.Named, format string) bool {
	names := []string{"MarshalText"}
	if format == "JSON" {
		names = append(names, "MarshalJSON")
	} else {
		names = append(names, "MarshalXML")
	}
	for _, valueType := range[]types.Type{named, types.NewPointer(named)} {
		methodSet := types.NewMethodSet(valueType)
		for index := range methodSet.Len() {
			name := methodSet.At(index).Obj().Name()
			for _, wanted := range names {
				if name == wanted {
					return true
				}
			}
		}
	}
	return false
}

func ignoredMarshalField(tag, format string) bool {
	name := strings.ToLower(format)
	value := reflect.StructTag(tag).Get(name)
	return value == "-" || strings.HasPrefix(value, "-,")
}
