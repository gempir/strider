//strider:ignore-file cognitive-complexity
package semantic

import (
	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type nilMapAssignmentCheck struct{}

func (nilMapAssignmentCheck) Meta() Meta {
	return Meta{
		Code:            "nil-map-assignment",
		Summary:         "detect assignments into maps proven to be nil",
		Explanation:     "Reading from a nil map is allowed, but assigning an entry to a nil map panics. Initialize the map with make or a map literal before writing.",
		GoodExample:     "values := make(map[string]int); values[key] = value",
		BadExample:      "var values map[string]int; values[key] = value",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (nilMapAssignmentCheck) Run(pass *Pass) {
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				update, ok := instruction.(*ssa.MapUpdate)
				if !ok || !isNilSSAConstant(flattenEquivalentPhi(update.Map)) {
					continue
				}
				pass.ReportPos(update.Pos(), "assignment to nil map will panic")
			}
		}
	}
}
