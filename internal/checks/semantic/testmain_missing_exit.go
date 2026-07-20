package semantic

import (
	"go/ast"
	"go/version"

	"github.com/gempir/strider/internal/diagnostic"
)

type testMainMissingExitRule struct{}

func (testMainMissingExitRule) Meta() Meta {
	return Meta{
		Code:            "test-main-missing-exit",
		Summary:         "detect legacy TestMain functions that lose the test exit code",
		Explanation:     "Before Go 1.15, a custom TestMain that called testing.M.Run had to pass its result to os.Exit or failed tests could appear successful. Go 1.15 and newer propagate the returned status automatically.",
		GoodExample:     "func TestMain(m *testing.M) { os.Exit(m.Run()) }",
		BadExample:      "func TestMain(m *testing.M) { m.Run() }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (testMainMissingExitRule) Run(pass *Pass) {
	if pass.GoVersion == "" || version.Compare(normalizeGoVersion(pass.GoVersion), "go1.15") >= 0 {
		return
	}
	for _, file := range pass.Files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || !isTestMainFunction(pass, function) {
				continue
			}
			parameter := pass.TypesInfo.Defs[function.Type.Params.List[0].Names[0]]
			callsRun, callsExit := false, false
			ast.Inspect(
				function.Body,
				func(node ast.Node) bool {
					call,
						ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					if isPackageFunction(pass.TypesInfo, call.Fun, "os", "Exit") {
						callsExit = true
					}
					selector,
						ok := call.Fun.(*ast.SelectorExpr)
					if !ok || selector.Sel.Name != "Run" {
						return true
					}
					identifier,
						ok := selector.X.(*ast.Ident)
					if ok && pass.TypesInfo.Uses[identifier] == parameter {
						callsRun = true
					}
					return true
				},
			)
			if callsRun && !callsExit {
				pass.Report(function, "TestMain must call os.Exit with the result of m.Run on this Go version")
			}
		}
	}
}

func isTestMainFunction(pass *Pass, function *ast.FuncDecl) bool {
	if function.Name.Name != "TestMain" || function.Recv != nil || function.Type.Params == nil || len(function.Type.Params.List) != 1 || len(function.Type.Params.List[0].Names) != 1 {
		return false
	}
	return isPointerToNamedType(pass.TypesInfo.TypeOf(function.Type.Params.List[0].Type), "testing", "M")
}

func (testMainMissingExitRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
