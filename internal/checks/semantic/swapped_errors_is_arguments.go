//strider:ignore-file cognitive-complexity,cyclomatic-complexity,modifies-parameter
package semantic

import (
	"go/token"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type swappedErrorsIsArgumentsCheck struct{}

func (swappedErrorsIsArgumentsCheck) Meta() Meta {
	return Meta{
		Code:            "swapped-errors-is-arguments",
		Summary:         "detect likely reversed errors.Is arguments",
		Explanation:     "errors.Is expects the error being inspected first and the target sentinel second. A package-level sentinel from another package in the first position, followed by a local error value, usually means the arguments were reversed.",
		GoodExample:     "errors.Is(err, io.EOF)",
		BadExample:      "errors.Is(io.EOF, err)",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (swappedErrorsIsArgumentsCheck) Run(pass *Pass) {
	calls := pass.argumentsByCallPosition()
	for _, call := range pass.staticCallsInPackage("errors") {
		if !isStaticFunction(call, "errors", "Is") || len(call.Common().Args) != 2 {
			continue
		}
		first := loadedGlobal(call.Common().Args[0])
		if first == nil || first.Pkg == nil || first.Pkg.Pkg == nil || first.Pkg.Pkg.Path() == pass.PackagePath {
			continue
		}
		second := loadedGlobal(call.Common().Args[1])
		if second != nil && second.Pkg != nil && second.Pkg.Pkg != nil && second.Pkg.Pkg.Path() != pass.PackagePath {
			continue
		}
		node := explicitCallArgument(calls[call.Pos()], 0, call.Pos())
		pass.Report(node, "errors.Is arguments appear reversed; inspect the local error first")
	}
}

func loadedGlobal(value ssa.Value) *ssa.Global {
	value = flattenSSAValue(value)
	load, ok := value.(*ssa.UnOp)
	if !ok || load.Op != token.MUL {
		return nil
	}
	global, _ := flattenSSAValue(load.X).(*ssa.Global)
	return global
}
