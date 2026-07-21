package semantic

import (
	"fmt"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
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
}

type discardedPureResultCheck struct{}

func (discardedPureResultCheck) Meta() Meta {
	return Meta{
		Code:            "discarded-pure-result",
		Summary:         "detect ignored results from functions without side effects",
		Explanation:     "Calling a function that has no side effects and then discarding all of its return values cannot affect program behavior. The call is either dead code or its result was meant to be used.",
		GoodExample:     "message := strings.TrimSpace(input)",
		BadExample:      "strings.TrimSpace(input)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (discardedPureResultCheck) Run(pass *Pass) {
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
				object, ok := calleeObject(callee)
				if !ok || !knownPureFunction(object) {
					continue
				}
				pass.ReportPos(call.Pos(), fmt.Sprintf("%s has no side effects and its return value is ignored", object.Name()))
			}
		}
	}
}

func calleeObject(function *ssa.Function) (*types.Func, bool) {
	if function == nil {
		return nil, false
	}
	object, ok := function.Object().(*types.Func)
	return object, ok
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
		if isPointerToNamedType(parameters.At(index).Type(), "testing", "B") {
			return true
		}
	}
	return false
}

func knownPureFunction(function *types.Func) bool {
	return pureStandardFunctions[function.FullName()]
}

func (discardedPureResultCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageSSA,
	}
}
