package semantic

import (
	"go/ast"
	"go/token"
	"go/version"
	"path/filepath"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type leakyTimeTickRule struct{}

func (leakyTimeTickRule) Meta() Meta {
	return Meta{
		Code:            "leaky-time-tick",
		Summary:         "detect time.Tick calls that leak on older Go versions",
		Explanation:     "Before Go 1.23, an unreferenced ticker could not be reclaimed unless it was stopped. time.Tick does not expose the ticker, so use time.NewTicker in functions that return. Go 1.23 and newer can reclaim unreferenced tickers.",
		GoodExample:     "ticker := time.NewTicker(time.Second)\ndefer ticker.Stop()",
		BadExample:      "ticks := time.Tick(time.Second)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (leakyTimeTickRule) Run(pass *Pass) {
	if pass.GoVersion == "" || version.Compare(normalizeGoVersion(pass.GoVersion), "go1.23") >= 0 || pass.Types.Name() == "main" {
		return
	}
	for _, function := range pass.Functions {
		if function.Syntax() == nil || !functionCanReturn(function) {
			continue
		}
		root := function.Syntax()
		ast.Inspect(
			root,
			func(node ast.Node) bool {
				if node == nil {
					return true
				}
				if node != root {
					switch node.(type) {
					case *ast.FuncDecl,
						*ast.FuncLit:
						return false
					}
				}
				call,
					ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) != 1 || !isPackageFunction(pass.TypesInfo, call.Fun, "time", "Tick") {
					return true
				}
				position := pass.FileSet.Position(call.Pos())
				if filepath.Ext(position.Filename) == ".go" && len(position.Filename) >= len("_test.go") && position.Filename[len(position.Filename)-len(
					"_test.go",
				):] == "_test.go" {
					return true
				}
				pass.Report(call, "time.Tick can leak its ticker on this Go version; use time.NewTicker and stop it when the function returns")
				return true
			},
		)
	}
}

func normalizeGoVersion(value string) string {
	if len(value) >= 2 && value[:2] == "go" {
		return value
	}
	return "go" + value
}

func functionCanReturn(function *ssa.Function) bool {
	for _, block := range function.Blocks {
		if len(block.Instrs) == 0 {
			continue
		}
		if _, ok := block.Instrs[len(block.Instrs)-1].(*ssa.Return); !ok {
			continue
		}
		if len(block.Preds) == 0 {
			return true
		}
		for _, predecessor := range block.Preds {
			if len(predecessor.Instrs) == 0 {
				return true
			}
			switch control := predecessor.Instrs[len(predecessor.Instrs)-1].(type) {
			case *ssa.Panic:
				continue
			case *ssa.If:
				if !receivesClosedTimeTick(control.Cond) {
					return true
				}
			default:
				return true
			}
		}
	}
	return false
}

func receivesClosedTimeTick(condition ssa.Value) bool {
	extract, ok := condition.(*ssa.Extract)
	if !ok || extract.Index != 1 {
		return false
	}
	receive, ok := extract.Tuple.(*ssa.UnOp)
	if !ok || receive.Op != token.ARROW || !receive.CommaOk {
		return false
	}
	call, ok := receive.X.(*ssa.Call)
	return ok && isStaticFunction(call, "time", "Tick")
}
