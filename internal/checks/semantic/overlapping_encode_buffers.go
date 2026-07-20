package semantic

import (
	"go/constant"
	"go/token"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type overlappingEncodeBuffersRule struct{}

type encodeBufferCall struct {
	destinationSSA    int
	sourceSSA         int
	destinationSource int
}

func (overlappingEncodeBuffersRule) Meta() Meta {
	return Meta{
		Code:            "overlapping-encode-buffers",
		Summary:         "detect overlapping source and destination encoding buffers",
		Explanation:     "Byte encoders that expand their input can overwrite source bytes before reading them when destination and source begin at the same memory. Use separate storage or a destination region proven not to overlap.",
		GoodExample:     "hex.Encode(destination, source)",
		BadExample:      "hex.Encode(buffer, buffer)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (overlappingEncodeBuffersRule) Run(pass *Pass) {
	calls := pass.argumentsByCallPosition()
	for _, packagePath := range []string{
		"encoding/ascii85",
		"encoding/base32",
		"encoding/base64",
		"encoding/hex",
	} {
		for _, call := range pass.staticCallsInPackage(packagePath) {
			descriptor, ok := encodeBufferDescriptor(call)
			if !ok || len(call.Common().Args) <= descriptor.sourceSSA {
				continue
			}
			destination := call.Common().Args[descriptor.destinationSSA]
			source := call.Common().Args[descriptor.sourceSSA]
			if !encodeBuffersOverlap(destination, source) {
				continue
			}
			node := explicitCallArgument(calls[call.Pos()], descriptor.destinationSource, call.Pos())
			pass.Report(node, "encoding destination overlaps the source buffer")
		}
	}
}

func encodeBufferDescriptor(call ssa.CallInstruction) (encodeBufferCall, bool) {
	if isStaticFunction(call, "encoding/ascii85", "Encode") || isStaticFunction(call, "encoding/hex", "Encode") {
		return encodeBufferCall{
			destinationSSA:    0,
			sourceSSA:         1,
			destinationSource: 0,
		}, true
	}
	callee := call.Common().StaticCallee()
	if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil || callee.Object().Name() != "Encode" {
		return encodeBufferCall{}, false
	}
	switch callee.Object().Pkg().Path() {
	case "encoding/base32", "encoding/base64":
		return encodeBufferCall{
			destinationSSA:    1,
			sourceSSA:         2,
			destinationSource: 0,
		}, true
	default:
		return encodeBufferCall{}, false
	}
}

func encodeBuffersOverlap(destination, source ssa.Value) bool {
	destination = flattenSSAValue(destination)
	source = flattenSSAValue(source)
	if isNilSSAConstant(destination) || isNilSSAConstant(source) {
		return false
	}
	if destination == source {
		return true
	}
	destinationSlice, destinationOK := destination.(*ssa.Slice)
	sourceSlice, sourceOK := source.(*ssa.Slice)
	if !destinationOK || !sourceOK || flattenSSAValue(destinationSlice.X) != flattenSSAValue(sourceSlice.X) {
		return false
	}
	return sameSliceBound(destinationSlice.Low, sourceSlice.Low)
}

func flattenSSAValue(value ssa.Value) ssa.Value {
	for {
		switch current := value.(type) {
		case *ssa.ChangeType:
			value = current.X
		case *ssa.Convert:
			value = current.X
		default:
			return value
		}
	}
}

func sameSliceBound(left, right ssa.Value) bool {
	if left == right {
		return true
	}
	leftConstant, leftOK := left.(*ssa.Const)
	rightConstant, rightOK := right.(*ssa.Const)
	return leftOK && rightOK && leftConstant.Value != nil && rightConstant.Value != nil && constant.Compare(leftConstant.Value, token.EQL, rightConstant.Value)
}

func (overlappingEncodeBuffersRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"encoding/ascii85",
			"encoding/base32",
			"encoding/base64",
			"encoding/hex",
		},
	}
}
