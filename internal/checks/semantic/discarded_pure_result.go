package semantic

import (
	"fmt"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

const (
	purityChecking purityState = iota + 1
	purityYes
	purityNo
)

var pureStandardFunctions = map[string]bool{
	"errors.New":                      true,
	"fmt.Errorf":                      true,
	"fmt.Sprintf":                     true,
	"fmt.Sprint":                      true,
	"sort.Reverse":                    true,
	"strings.Map":                     true,
	"strings.Repeat":                  true,
	"strings.Replace":                 true,
	"strings.Title":                   true,
	"strings.ToLower":                 true,
	"strings.ToLowerSpecial":          true,
	"strings.ToTitle":                 true,
	"strings.ToTitleSpecial":          true,
	"strings.ToUpper":                 true,
	"strings.ToUpperSpecial":          true,
	"strings.Trim":                    true,
	"strings.TrimFunc":                true,
	"strings.TrimLeft":                true,
	"strings.TrimLeftFunc":            true,
	"strings.TrimPrefix":              true,
	"strings.TrimRight":               true,
	"strings.TrimRightFunc":           true,
	"strings.TrimSpace":               true,
	"strings.TrimSuffix":              true,
	"(*net/http.Request).WithContext": true,
	"time.Now":                        true,
	"time.Parse":                      true,
	"time.ParseInLocation":            true,
	"time.Unix":                       true,
	"time.UnixMicro":                  true,
	"time.UnixMilli":                  true,
	"(time.Time).Add":                 true,
	"(time.Time).AddDate":             true,
	"(time.Time).After":               true,
	"(time.Time).Before":              true,
	"(time.Time).Clock":               true,
	"(time.Time).Compare":             true,
	"(time.Time).Date":                true,
	"(time.Time).Day":                 true,
	"(time.Time).Equal":               true,
	"(time.Time).Format":              true,
	"(time.Time).GoString":            true,
	"(time.Time).GobEncode":           true,
	"(time.Time).Hour":                true,
	"(time.Time).ISOWeek":             true,
	"(time.Time).In":                  true,
	"(time.Time).IsDST":               true,
	"(time.Time).IsZero":              true,
	"(time.Time).Local":               true,
	"(time.Time).Location":            true,
	"(time.Time).MarshalBinary":       true,
	"(time.Time).MarshalJSON":         true,
	"(time.Time).MarshalText":         true,
	"(time.Time).Minute":              true,
	"(time.Time).Month":               true,
	"(time.Time).Nanosecond":          true,
	"(time.Time).Round":               true,
	"(time.Time).Second":              true,
	"(time.Time).String":              true,
	"(time.Time).Sub":                 true,
	"(time.Time).Truncate":            true,
	"(time.Time).UTC":                 true,
	"(time.Time).Unix":                true,
	"(time.Time).UnixMicro":           true,
	"(time.Time).UnixMilli":           true,
	"(time.Time).UnixNano":            true,
	"(time.Time).Weekday":             true,
	"(time.Time).Year":                true,
	"(time.Time).YearDay":             true,
	"(time.Time).Zone":                true,
	"(time.Time).ZoneBounds":          true,
}

type discardedPureResultRule struct{}

type purityState uint8

type purityChecker struct {
	pass  *Pass
	state map[*ssa.Function]purityState
}

func (discardedPureResultRule) Meta() Meta {
	return Meta{
		Code:            "discarded-pure-result",
		Summary:         "detect ignored results from functions without side effects",
		Explanation:     "Calling a function that has no side effects and then discarding all of its return values cannot affect program behavior. The call is either dead code or its result was meant to be used.",
		GoodExample:     "message := strings.TrimSpace(input)",
		BadExample:      "strings.TrimSpace(input)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (discardedPureResultRule) Run(pass *Pass) {
	purity := newPurityChecker(pass)
	for _, function := range pass.Functions {
		if benchmarkHelper(function) {
			continue
		}
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(*ssa.Call)
				if !ok || call.Referrers() == nil || hasNonDebugInstructions(*call.Referrers()) {
					continue
				}
				callee := call.Common().StaticCallee()
				if callee == nil || callee.Object() == nil || !purity.pure(callee) {
					continue
				}
				pass.Report(positionNode{
					position: call.Pos(),
				}, fmt.Sprintf("%s has no side effects and its return value is ignored", callee.Object().Name()))
			}
		}
	}
}

func hasNonDebugInstructions(instructions []ssa.Instruction) bool {
	for _, instruction := range instructions {
		if _, debug := instruction.(*ssa.DebugRef); !debug {
			return true
		}
	}
	return false
}

func benchmarkHelper(function *ssa.Function) bool {
	if function == nil || function.Signature == nil {
		return false
	}
	parameters := function.Signature.Params()
	for index := range parameters.Len() {
		pointer, ok := types.Unalias(parameters.At(index).Type()).(*types.Pointer)
		if !ok {
			continue
		}
		named, ok := types.Unalias(pointer.Elem()).(*types.Named)
		if ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "testing" && named.Obj().Name() == "B" {
			return true
		}
	}
	return false
}

func newPurityChecker(pass *Pass) *purityChecker {
	return &purityChecker{
		pass:  pass,
		state: make(map[*ssa.Function]purityState),
	}
}

func (checker *purityChecker) pure(function *ssa.Function) bool {
	if function == nil || function.Object() == nil {
		return false
	}
	object, ok := function.Object().(*types.Func)
	if !ok {
		return false
	}
	if knownPureFunction(object) {
		return true
	}
	if function.Pkg != checker.pass.SSAPackage {
		return false
	}
	switch checker.state[function] {
	case purityYes:
		return true
	case purityNo:
		return false
	case purityChecking:
		return false
	}
	checker.state[function] = purityChecking
	pure := checker.inspect(function)
	if pure {
		checker.state[function] = purityYes
	} else {
		checker.state[function] = purityNo
	}
	return pure
}

func (checker *purityChecker) inspect(function *ssa.Function) bool {
	if function.Blocks == nil || function.Signature.Results().Len() == 0 {
		return false
	}
	for _, parameter := range function.Params {
		if !basicPurityType(parameter.Type()) {
			return false
		}
	}
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			switch instruction := instruction.(type) {
			case *ssa.Call:
				if !checker.pureCall(instruction.Common(), function) {
					return false
				}
			case *ssa.Defer:
				if !checker.pureCall(instruction.Common(), function) {
					return false
				}
			case *ssa.Select, *ssa.Send, *ssa.Go, *ssa.Panic, *ssa.MapUpdate, *ssa.MakeMap, *ssa.MakeChan, *ssa.MakeSlice, *ssa.MakeClosure:
				return false
			case *ssa.Store:
				if !stackAddress(instruction.Addr) {
					return false
				}
			case *ssa.FieldAddr:
				if !stackAddress(instruction.X) {
					return false
				}
			case *ssa.Alloc:
				if instruction.Heap {
					return false
				}
			case *ssa.UnOp:
				if instruction.Op == token.ARROW || (instruction.Op == token.MUL && !stackAddress(instruction.X)) {
					return false
				}
			}
		}
	}
	return true
}

func (checker *purityChecker) pureCall(common *ssa.CallCommon, current *ssa.Function) bool {
	if common == nil || common.IsInvoke() {
		return false
	}
	if builtin, ok := common.Value.(*ssa.Builtin); ok {
		return builtin.Name() == "len" || builtin.Name() == "cap"
	}
	callee := common.StaticCallee()
	if callee == nil {
		return false
	}
	return callee == current || checker.pure(callee)
}

func basicPurityType(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	switch underlying := types.Unalias(valueType).Underlying().(type) {
	case *types.Basic:
		return true
	case *types.Struct:
		for index := range underlying.NumFields() {
			if !basicPurityType(underlying.Field(index).Type()) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func stackAddress(value ssa.Value) bool {
	switch value := value.(type) {
	case *ssa.Alloc:
		return !value.Heap
	case *ssa.FieldAddr:
		return stackAddress(value.X)
	case *ssa.IndexAddr:
		return stackAddress(value.X)
	default:
		return false
	}
}

func knownPureFunction(function *types.Func) bool {
	return pureStandardFunctions[function.FullName()]
}
