package semantic

import (
	"fmt"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type testingFatalInGoroutineRule struct {}

func (testingFatalInGoroutineRule) Meta() Meta {
	return Meta{
		Code: "testing-fatal-in-goroutine",
		Summary: "detect test termination methods called from child goroutines",
		Explanation: "testing.T and testing.B methods that terminate or skip execution must run in the same goroutine as the test. Calling Fatal, FailNow, Skip, or related methods from a child goroutine does not stop the test correctly.",
		GoodExample: "if err := work(); err != nil { t.Fatal(err) }",
		BadExample: "go func() { t.Fatal(\"failed\") }()",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (testingFatalInGoroutineRule) Run(pass *Pass) {
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				started, ok := instruction.(*ssa.Go)
				if !ok {
					continue
				}
				target := started.Common().StaticCallee()
				if target == nil || target.Blocks == nil {
					continue
				}
				method := terminatingTestMethod(target)
				if method == "" {
					continue
				}
				pass.Report(positionNode{position: started.Pos()}, fmt.Sprintf("%s must be called from the test goroutine, not a child goroutine", method))
			}
		}
	}
}

func terminatingTestMethod(function *ssa.Function) string {
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			call, ok := instruction.(ssa.CallInstruction)
			if !ok {
				continue
			}
			callee := call.Common().StaticCallee()
			if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil || callee.Object().Pkg().Path() != "testing" {
				continue
			}
			functionObject, ok := callee.Object().(*types.Func)
			if !ok {
				continue
			}
			signature, _ := functionObject.Type().(*types.Signature)
			if signature == nil || signature.Recv() == nil {
				continue
			}
			switch functionObject.Name() {
			case "FailNow", "Fatal", "Fatalf", "SkipNow", "Skip", "Skipf":
				return functionObject.Name()
			}
		}
	}
	return ""
}
