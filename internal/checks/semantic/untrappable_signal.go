package semantic

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type untrappableSignalCheck struct{}

func (untrappableSignalCheck) Meta() Meta {
	return Meta{
		Code:            "untrappable-signal",
		Summary:         "detect attempts to handle signals that cannot be trapped",
		Explanation:     "SIGKILL and SIGSTOP are handled directly by Unix-like kernels and are never delivered to a process. Passing either signal to os/signal notification APIs cannot work.",
		GoodExample:     "signal.Notify(ch, syscall.SIGTERM)",
		BadExample:      "signal.Notify(ch, os.Kill)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (untrappableSignalCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
		},
		func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !isSignalRegistration(pass.TypesInfo, call.Fun) {
				return true
			}
			for _, argument := range call.Args {
				signal := unwrapSignalConversion(pass.TypesInfo, argument)
				name := untrappableSignalName(pass.TypesInfo, signal)
				if name == "" {
					continue
				}
				pass.Report(argument, fmt.Sprintf("%s cannot be trapped by a process", name))
			}
			return true
		},
	)
}

func isSignalRegistration(info *types.Info, expression ast.Expr) bool {
	function := calledFunction(info, expression)
	if function == nil || function.Pkg() == nil || function.Pkg().Path() != "os/signal" {
		return false
	}
	switch function.Name() {
	case "Ignore", "Notify", "Reset":
		return true
	default:
		return false
	}
}

func unwrapSignalConversion(info *types.Info, expression ast.Expr) ast.Expr {
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return expression
	}
	typeName, ok := info.Uses[typeIdentifier(call.Fun)].(*types.TypeName)
	if !ok || typeName.Pkg() == nil || typeName.Pkg().Path() != "os" || typeName.Name() != "Signal" {
		return expression
	}
	return call.Args[0]
}

func typeIdentifier(expression ast.Expr) *ast.Ident {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression
	case *ast.SelectorExpr:
		return expression.Sel
	case *ast.ParenExpr:
		return typeIdentifier(expression.X)
	default:
		return nil
	}
}

func untrappableSignalName(info *types.Info, expression ast.Expr) string {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	object := info.Uses[selector.Sel]
	if object == nil || object.Pkg() == nil {
		return ""
	}
	switch object.Pkg().Path() + "." + object.Name() {
	case "os.Kill":
		return "os.Kill"
	case "syscall.SIGKILL":
		return "syscall.SIGKILL"
	case "syscall.SIGSTOP":
		return "syscall.SIGSTOP"
	default:
		return ""
	}
}

func (untrappableSignalCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
