package semantic

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type unsupportedBinaryWriteRule struct {}

func (unsupportedBinaryWriteRule) Meta() Meta {
	return Meta{
		Code: "unsupported-binary-write",
		Summary: "detect unsupported encoding/binary.Write values",
		Explanation: "encoding/binary can only serialize fixed-size values; architecture-sized integers, strings, maps, channels, functions, pointers in aggregates, and other variable-size values are unsupported.",
		GoodExample: "binary.Write(writer, binary.LittleEndian, uint32(value))",
		BadExample: "binary.Write(writer, binary.LittleEndian, value) // value is int",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (unsupportedBinaryWriteRule) Run(pass *Pass) {
	calls := pass.argumentsByCallPosition()
	for _, call := range pass.staticCallsInPackage("encoding/binary") {
		if !isStaticFunction(call, "encoding/binary", "Write") || len(call.Common().Args) < 3 {
			continue
		}
		value := unwrapSSAValue(call.Common().Args[2])
		if canBinaryMarshal(value.Type()) {
			continue
		}
		node := binaryWriteDataNode(calls[call.Pos()], call.Pos())
		pass.Report(node, fmt.Sprintf("value of type %s cannot be used with binary.Write", value.Type()))
	}
}

func unwrapSSAValue(value ssa.Value) ssa.Value {
	switch value := value.(type) {
	case *ssa.MakeInterface:
		return unwrapSSAValue(value.X)
	case *ssa.ChangeInterface:
		return unwrapSSAValue(value.X)
	default:
		return value
	}
}

func canBinaryMarshal(valueType types.Type) bool {
	typeToCheck := valueType.Underlying()
	if pointer, ok := typeToCheck.(*types.Pointer); ok {
		typeToCheck = pointer.Elem().Underlying()
	}
	if withElement, ok := types.Unalias(typeToCheck).(interface {
		Elem() types.Type
	}); ok {
		if _, pointer := withElement.(*types.Pointer); !pointer {
			typeToCheck = withElement.Elem()
		}
	}
	return validEncodingBinaryType(typeToCheck)
}

func validEncodingBinaryType(valueType types.Type) bool {
	switch valueType := valueType.Underlying().(type) {
	case *types.Basic:
		switch valueType.Kind() {
		case types.Bool, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.Int8, types.Int16, types.Int32, types.Int64, types.Float32, types.Float64, types.Complex64, types.Complex128, types.Invalid:
			return true
		default:
			return false
		}
	case *types.Struct:
		for index := range valueType.NumFields() {
			if !validEncodingBinaryType(valueType.Field(index).Type()) {
				return false
			}
		}
		return true
	case *types.Array:
		return validEncodingBinaryType(valueType.Elem())
	case *types.Interface:
		return true
	default:
		return false
	}
}

func binaryWriteDataNode(arguments []ast.Node, position token.Pos) ast.Node {
	if len(arguments) > 2 {
		return arguments[2]
	}
	if len(arguments) != 0 {
		return arguments[0]
	}
	return positionNode{position: position}
}
