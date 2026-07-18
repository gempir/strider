package semantic

import (
	"fmt"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type unexportedSerializationFieldsRule struct{}

type serializationCall struct {
	format         string
	direction      string
	ssaArgument    int
	sourceArgument int
}

func (unexportedSerializationFieldsRule) Meta() Meta {
	return Meta{
		Code:            "unexported-serialization-fields",
		Summary:         "detect serialization of structs with no exported fields",
		Explanation:     "The standard JSON and XML packages ignore unexported struct fields. Marshaling a non-empty struct with no exported fields produces empty data, and unmarshaling into it cannot populate anything, unless the type defines custom serialization behavior.",
		GoodExample:     "json.Marshal(struct{ Name string }{Name: name})",
		BadExample:      "json.Marshal(struct{ name string }{name: name})",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (unexportedSerializationFieldsRule) Run(pass *Pass) {
	calls := pass.argumentsByCallPosition()
	for _, packagePath := range []string{"encoding/json", "encoding/xml"} {
		for _, call := range pass.staticCallsInPackage(packagePath) {
			descriptor, ok := serializationDescriptor(call)
			if !ok || len(call.Common().Args) <= descriptor.ssaArgument {
				continue
			}
			value := unwrapSSAValue(call.Common().Args[descriptor.ssaArgument])
			if !serializationFieldsInvisible(value.Type(), descriptor) {
				continue
			}
			node := explicitCallArgument(calls[call.Pos()], descriptor.sourceArgument, call.Pos())
			pass.Report(
				node,
				fmt.Sprintf(
					"%s %s sees no exported fields in %s and no custom serialization method",
					descriptor.format,
					descriptor.direction,
					types.TypeString(value.Type(), types.RelativeTo(pass.Types)),
				),
			)
		}
	}
}

func serializationDescriptor(call ssa.CallInstruction) (serializationCall, bool) {
	if descriptor, ok := marshalDescriptor(call); ok {
		return serializationCall{
			format:         descriptor.format,
			direction:      "marshaling",
			ssaArgument:    descriptor.ssaArgument,
			sourceArgument: descriptor.sourceArgument,
		}, true
	}
	if descriptor, ok := unmarshalDescriptor(call); ok {
		format := "JSON"
		callee := call.Common().StaticCallee()
		if callee != nil && callee.Object() != nil && callee.Object().Pkg() != nil && callee.Object().Pkg().Path() == "encoding/xml" {
			format = "XML"
		}
		return serializationCall{
			format:         format,
			direction:      "unmarshaling",
			ssaArgument:    descriptor.ssaArgument,
			sourceArgument: descriptor.sourceArgument,
		}, true
	}
	return serializationCall{}, false
}

func serializationFieldsInvisible(valueType types.Type, descriptor serializationCall) bool {
	original := types.Unalias(valueType)
	structureType := original
	for {
		pointer, ok := structureType.Underlying().(*types.Pointer)
		if !ok {
			break
		}
		structureType = types.Unalias(pointer.Elem())
	}
	structure, ok := structureType.Underlying().(*types.Struct)
	if !ok || structure.NumFields() == 0 || hasVisibleSerializationField(
		structure,
		make(map[*types.Struct]bool),
	) {
		return false
	}
	return !hasCustomSerializationMethod(original, descriptor)
}

func hasVisibleSerializationField(structure *types.Struct, seen map[*types.Struct]bool) bool {
	if seen[structure] {
		return false
	}
	seen[structure] = true
	for index := range structure.NumFields() {
		field := structure.Field(index)
		if field.Exported() {
			return true
		}
		if !field.Anonymous() {
			continue
		}
		fieldType := field.Type()
		if pointer, ok := fieldType.Underlying().(*types.Pointer); ok {
			fieldType = pointer.Elem()
		}
		if embedded, ok := fieldType.Underlying().(*types.Struct); ok && hasVisibleSerializationField(
			embedded,
			seen,
		) {
			return true
		}
	}
	return false
}

func hasCustomSerializationMethod(valueType types.Type, descriptor serializationCall) bool {
	names := []string{"MarshalText"}
	if descriptor.direction == "unmarshaling" {
		names = []string{"UnmarshalText"}
		if descriptor.format == "JSON" {
			names = append(names, "UnmarshalJSON")
		} else {
			names = append(names, "UnmarshalXML")
		}
	} else if descriptor.format == "JSON" {
		names = append(names, "MarshalJSON")
	} else {
		names = append(names, "MarshalXML")
	}
	candidates := make([]types.Type, 0, 4)
	current := types.Unalias(valueType)
	for {
		candidates = append(candidates, current)
		pointer, ok := current.Underlying().(*types.Pointer)
		if !ok {
			candidates = append(candidates, types.NewPointer(current))
			break
		}
		current = types.Unalias(pointer.Elem())
	}
	seen := make(map[types.Type]bool)
	for _, candidate := range candidates {
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		methodSet := types.NewMethodSet(candidate)
		for _, name := range names {
			if methodSet.Lookup(nil, name) != nil {
				return true
			}
		}
	}
	return false
}
