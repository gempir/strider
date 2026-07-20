package semantic

import (
	"fmt"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type externalCallInLoopRule struct{}

func (externalCallInLoopRule) Meta() Meta {
	return Meta{
		Code:            "external-call-in-loop",
		Summary:         "detect synchronous SQL and HTTP calls inside loops",
		Explanation:     "A database query or HTTP request issued synchronously on each loop iteration creates serial network round trips and commonly indicates an N+1 access pattern. Batch work before the loop when possible. Calls inside nested function literals are analyzed in their own control-flow graph and are not attributed to the enclosing loop.",
		GoodExample:     "rows, err := db.QueryContext(ctx, batchQuery, ids)\nfor rows.Next() { /* map results in memory */ }",
		BadExample:      "for _, id := range ids { row := db.QueryRowContext(ctx, query, id); _ = row }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (externalCallInLoopRule) Run(pass *Pass) {
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			if !ssaBlockInCycle(block) {
				continue
			}
			for _, instruction := range block.Instrs {
				call, ok := instruction.(*ssa.Call)
				if !ok {
					continue
				}
				description := knownExternalLoopCall(call.Common())
				if description == "" {
					continue
				}
				pass.ReportPos(
					call.Pos(),
					fmt.Sprintf("%s is called synchronously inside a loop; batch or move the external operation outside the loop", description),
				)
			}
		}
	}
}

func knownExternalLoopCall(call *ssa.CallCommon) string {
	callee := call.StaticCallee()
	if callee == nil {
		return ""
	}
	function, ok := callee.Object().(*types.Func)
	if !ok || function.Pkg() == nil {
		return ""
	}
	packagePath := function.Pkg().Path()
	name := function.Name()
	switch packagePath {
	case "database/sql":
		switch name {
		case "Exec", "ExecContext", "Query", "QueryContext", "QueryRow", "QueryRowContext":
			return "database/sql." + name
		}
	case "net/http":
		switch name {
		case "Do", "Get", "Head", "Post", "PostForm":
			return "net/http." + name
		}
	}
	return ""
}

func (externalCallInLoopRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageSSA,
	}
}
