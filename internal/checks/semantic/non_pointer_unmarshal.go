package semantic

import (
	"fmt"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type nonPointerUnmarshalRule struct{}

type unmarshalCall struct {
	name           string
	ssaArgument    int
	sourceArgument int
}

func (nonPointerUnmarshalRule) Meta() Meta {
	return Meta{
		Code:            "non-pointer-unmarshal",
		Summary:         "detect non-pointer unmarshal destinations",
		Explanation:     "JSON and XML unmarshalling and decoding APIs require a pointer destination so they can populate the provided value.",
		GoodExample:     "json.Unmarshal(data, &value)",
		BadExample:      "json.Unmarshal(data, value)",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (nonPointerUnmarshalRule) Run(pass *Pass) {
	calls := pass.argumentsByCallPosition()
	for _, packagePath := range []string{
		"encoding/json",
		"encoding/xml",
	} {
		for _, call := range pass.staticCallsInPackage(packagePath) {
			descriptor, ok := unmarshalDescriptor(call)
			if !ok || len(call.Common().Args) <= descriptor.ssaArgument {
				continue
			}
			value := unwrapSSAValue(call.Common().Args[descriptor.ssaArgument])
			if isPointerOrInterface(value.Type()) {
				continue
			}
			node := explicitCallArgument(calls[call.Pos()], descriptor.sourceArgument, call.Pos())
			pass.Report(node, fmt.Sprintf("%s expects to unmarshal into a pointer, but the provided value is not a pointer", descriptor.name))
		}
	}
}

func unmarshalDescriptor(call ssa.CallInstruction) (unmarshalCall, bool) {
	callee := call.Common().StaticCallee()
	if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil {
		return unmarshalCall{}, false
	}
	path, name := callee.Object().Pkg().Path(), callee.Object().Name()
	signature, _ := callee.Object().Type().(*types.Signature)
	isMethod := signature != nil && signature.Recv() != nil
	if !isMethod && name == "Unmarshal" {
		switch path {
		case "encoding/json":
			return unmarshalCall{
				name:           "json.Unmarshal",
				ssaArgument:    1,
				sourceArgument: 1,
			}, true
		case "encoding/xml":
			return unmarshalCall{
				name:           "xml.Unmarshal",
				ssaArgument:    1,
				sourceArgument: 1,
			}, true
		}
	}
	if isMethod && path == "encoding/json" && name == "Decode" {
		return unmarshalCall{
			name:           "Decode",
			ssaArgument:    1,
			sourceArgument: 0,
		}, true
	}
	if isMethod && path == "encoding/xml" && (name == "Decode" || name == "DecodeElement") {
		return unmarshalCall{
			name:           name,
			ssaArgument:    1,
			sourceArgument: 0,
		}, true
	}
	return unmarshalCall{}, false
}

func isPointerOrInterface(valueType types.Type) bool {
	switch valueType.Underlying().(type) {
	case *types.Pointer, *types.Interface:
		return true
	default:
		return false
	}
}

func (nonPointerUnmarshalRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"encoding/json",
			"encoding/xml",
		},
	}
}
