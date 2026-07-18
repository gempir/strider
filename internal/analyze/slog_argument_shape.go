package analyze

import (
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type slogArgumentShapeRule struct {}

func (slogArgumentShapeRule) Meta() Meta {
	return Meta{
		Code: "slog-argument-shape",
		Summary: "detect malformed or inconsistent log/slog arguments",
		Explanation: "The loose arguments accepted by log/slog alternate string keys and values, while slog.Attr values form a separate typed style. Odd tails, keys whose dynamic type is not exactly string, and calls that mix Attr values with loose pairs produce malformed or needlessly inconsistent records.",
		GoodExample: `slog.Info("request", "method", method, "status", status)`,
		BadExample: `slog.Info("request", 42, method, "status")`,
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (slogArgumentShapeRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				call,
				ok := node.(*ast.CallExpr)
				if !ok || call.Ellipsis.IsValid() {
					return true
				}
				function := calledFunction(pass.TypesInfo, call.Fun)
				start,
				ok := slogLooseArgumentStart(function)
				if !ok || start > len(call.Args) {
					return true
				}
				tail := call.Args[start:]
				if len(tail) == 0 {
					return true
				}

				attrs := 0
				for _,
				argument := range tail {
					if isSlogAttr(pass.TypesInfo.TypeOf(argument)) {
						attrs++
					}
				}
				if attrs != 0 && attrs != len(tail) {
					pass.Report(
						call,
						"do not mix slog.Attr values with loose key/value arguments in one slog call",
					)
					return true
				}
				if attrs == len(tail) {
					return true
				}
				if len(tail) % 2 != 0 {
					pass.Report(
						tail[len(tail) - 1],
						"slog key/value arguments must contain an even number of elements",
					)
					return true
				}
				for index := 0; index < len(tail); index += 2 {
					if exactSlogStringKey(pass.TypesInfo.TypeOf(tail[index])) {
						continue
					}
					pass.Report(tail[index], "slog key must have the built-in string type")
					return true
				}
				return true
			},
		)
	}
}

func slogLooseArgumentStart(function *types.Func) (int, bool) {
	if function == nil || function.Pkg() == nil || function.Pkg().Path() != "log/slog" {
		return 0, false
	}
	switch function.Name() {
	case "Debug", "Info", "Warn", "Error", "Group":
		return 1, true
	case "DebugContext", "InfoContext", "WarnContext", "ErrorContext":
		return 2, true
	case "Log":
		return 3, true
	case "With":
		signature, _ := function.Type().(*types.Signature)
		return 0, signature != nil && signature.Recv() != nil
	default:
		return 0, false
	}
}

func isSlogAttr(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	named, ok := types.Unalias(valueType).(*types.Named)
	return ok && named.Obj() != nil && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "log/slog" && named.Obj().Name() == "Attr"
}

func exactSlogStringKey(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	basic, ok := types.Unalias(valueType).(*types.Basic)
	return ok && basic.Info() & types.IsString != 0
}
