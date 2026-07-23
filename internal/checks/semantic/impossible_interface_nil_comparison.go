//strider:ignore-file cognitive-complexity,cyclomatic-complexity,modifies-parameter,redefines-builtin-id
package semantic

import (
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

const (
	interfaceNilUnknown interfaceNilProof = iota
	interfaceNilConcrete
	interfaceNilFromCall
)

type impossibleInterfaceNilComparisonCheck struct{}

type interfaceNilProof uint8

type interfaceResultKey struct {
	function *ssa.Function
	index    int
}

type interfaceNilChecker struct {
	pass     *Pass
	checking map[interfaceResultKey]bool
	results  map[interfaceResultKey]interfaceNilProof
}

func (impossibleInterfaceNilComparisonCheck) Meta() Meta {
	return Meta{
		Code:            "impossible-interface-nil-comparison",
		Summary:         "detect interface comparisons made non-nil by a concrete dynamic type",
		Explanation:     "An interface is nil only when both its dynamic type and value are absent. Storing a typed nil pointer in an interface gives it a concrete dynamic type, so the interface itself is non-nil.",
		GoodExample:     "func result(ok bool) error { if !ok { return nil }; return &problem{} }",
		BadExample:      "func result() error { var problem *Problem; return problem }",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (impossibleInterfaceNilComparisonCheck) Run(pass *Pass) {
	checker := newInterfaceNilChecker(pass)
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				binary, ok := instruction.(*ssa.BinOp)
				if !ok || binary.Op != token.EQL && binary.Op != token.NEQ {
					continue
				}
				interfaceValue, ok := interfaceNilComparisonOperands(binary.X, binary.Y)
				if !ok {
					interfaceValue, ok = interfaceNilComparisonOperands(binary.Y, binary.X)
				}
				if !ok {
					continue
				}
				proof := checker.neverNil(interfaceValue, make(map[ssa.Value]bool))
				if proof == interfaceNilUnknown || proof == interfaceNilFromCall && strings.HasSuffix(pass.FileSet.Position(binary.Pos()).Filename, "_test.go") {
					continue
				}
				truth := "never"
				if binary.Op == token.NEQ {
					truth = "always"
				}
				pass.ReportPos(binary.Pos(), "interface has a concrete dynamic type; this comparison is "+truth+" true")
			}
		}
	}
}

func interfaceNilComparisonOperands(interfaceValue, nilValue ssa.Value) (ssa.Value, bool) {
	if !isNilSSAConstant(nilValue) || interfaceValue == nil {
		return nil, false
	}
	if _, parameter := types.Unalias(interfaceValue.Type()).(*types.TypeParam); parameter {
		return nil, false
	}
	_, ok := types.Unalias(interfaceValue.Type()).Underlying().(*types.Interface)
	return interfaceValue, ok
}

func newInterfaceNilChecker(pass *Pass) *interfaceNilChecker {
	return &interfaceNilChecker{
		pass:     pass,
		checking: make(map[interfaceResultKey]bool),
		results:  make(map[interfaceResultKey]interfaceNilProof),
	}
}

func (checker *interfaceNilChecker) neverNil(value ssa.Value, seen map[ssa.Value]bool) interfaceNilProof {
	if value == nil || seen[value] {
		return interfaceNilUnknown
	}
	seen[value] = true
	switch value := value.(type) {
	case *ssa.MakeInterface:
		if _, parameter := types.Unalias(value.X.Type()).(*types.TypeParam); parameter {
			return interfaceNilUnknown
		}
		if _, nestedInterface := types.Unalias(value.X.Type()).Underlying().(*types.Interface); nestedInterface {
			return interfaceNilUnknown
		}
		return interfaceNilConcrete
	case *ssa.ChangeInterface:
		return checker.neverNil(value.X, seen)
	case *ssa.Phi:
		proof := interfaceNilConcrete
		for _, edge := range value.Edges {
			edgeProof := checker.neverNil(edge, copyValueSet(seen))
			if edgeProof == interfaceNilUnknown {
				return interfaceNilUnknown
			}
			if edgeProof == interfaceNilFromCall {
				proof = interfaceNilFromCall
			}
		}
		return proof
	case *ssa.Call:
		callee := value.Common().StaticCallee()
		if callee != nil && checker.resultNeverNil(callee, 0) != interfaceNilUnknown {
			return interfaceNilFromCall
		}
	case *ssa.Extract:
		call, ok := flattenEquivalentPhi(value.Tuple).(*ssa.Call)
		if ok {
			callee := call.Common().StaticCallee()
			if callee != nil && checker.resultNeverNil(callee, value.Index) != interfaceNilUnknown {
				return interfaceNilFromCall
			}
		}
	}
	return interfaceNilUnknown
}

func (checker *interfaceNilChecker) resultNeverNil(function *ssa.Function, index int) interfaceNilProof {
	key := interfaceResultKey{
		function: function,
		index:    index,
	}
	if proof, known := checker.results[key]; known {
		return proof
	}
	if function == nil || function.Pkg != checker.pass.SSAPackage || function.Blocks == nil || function.Signature == nil || index >= function.Signature.Results().Len() || checker.checking[key] {
		return interfaceNilUnknown
	}
	checker.checking[key] = true
	defer delete(checker.checking, key)
	foundReturn := false
	proof := interfaceNilConcrete
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			returned, ok := instruction.(*ssa.Return)
			if !ok || index >= len(returned.Results) {
				continue
			}
			foundReturn = true
			resultProof := checker.neverNil(returned.Results[index], make(map[ssa.Value]bool))
			if resultProof == interfaceNilUnknown {
				checker.results[key] = interfaceNilUnknown
				return interfaceNilUnknown
			}
			if resultProof == interfaceNilFromCall {
				proof = interfaceNilFromCall
			}
		}
	}
	if !foundReturn {
		proof = interfaceNilUnknown
	}
	checker.results[key] = proof
	return proof
}

func copyValueSet(values map[ssa.Value]bool) map[ssa.Value]bool {
	copy := make(map[ssa.Value]bool, len(values))
	for value := range values {
		copy[value] = true
	}
	return copy
}
