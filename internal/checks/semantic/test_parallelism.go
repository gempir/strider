package semantic

import (
	"go/ast"
	"go/types"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gempir/strider/internal/diagnostic"
)

type testParallelismCheck struct{}

func (testParallelismCheck) Meta() Meta {
	return Meta{
		Code:            "test-parallelism",
		Summary:         "identify tests and direct subtests that can opt into parallel execution",
		Explanation:     "Independent tests can call t.Parallel to reduce suite latency. This advisory check skips tests that already opt in or visibly mutate process-global state, including environment and working-directory changes.",
		GoodExample:     "func TestLoad(t *testing.T) { t.Parallel(); checkLoad(t) }",
		BadExample:      "func TestLoad(t *testing.T) { checkLoad(t) }",
		DefaultSeverity: diagnostic.SeverityNote,
	}
}

func (testParallelismCheck) Run(pass *Pass) {
	for _, file := range pass.Files {
		filename := pass.FileSet.Position(file.Pos()).Filename
		if !strings.HasSuffix(filename, "_test.go") {
			continue
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || !testFunctionName(function.Name.Name) {
				continue
			}
			parameter := testingTParameter(pass, function.Type)
			if parameter == nil || function.Body == nil || hasTestingParallelCall(pass, function.Body, parameter) || hasUnsafeParallelTestState(pass, function.Body) {
				continue
			}
			pass.Report(function.Name, "consider calling t.Parallel() when this test begins")
		}
	}
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
		},
		func(node ast.Node) bool {
			call := node.(*ast.CallExpr)
			file := pass.File(call.Pos())
			if file == nil || !strings.HasSuffix(pass.FileSet.Position(file.Pos()).Filename, "_test.go") || !isTestingMethod(pass, call.Fun, "Run") || len(call.Args) != 2 {
				return true
			}
			literal,
				ok := ast.Unparen(call.Args[1]).(*ast.FuncLit)
			if !ok {
				return true
			}
			parameter := testingTParameter(pass, literal.Type)
			if parameter == nil || literal.Body == nil || hasTestingParallelCall(pass, literal.Body, parameter) || hasUnsafeParallelTestState(pass, literal.Body) {
				return true
			}
			pass.Report(literal, "consider calling t.Parallel() when this subtest begins")
			return true
		},
	)
}

func testFunctionName(name string) bool {
	if !strings.HasPrefix(name, "Test") {
		return false
	}
	suffix := strings.TrimPrefix(name, "Test")
	if suffix == "" {
		return true
	}
	first, _ := utf8.DecodeRuneInString(suffix)
	return !unicode.IsLower(first)
}

func testingTParameter(pass *Pass, function *ast.FuncType) *types.Var {
	if function == nil || function.Params == nil || len(function.Params.List) != 1 || len(function.Params.List[0].Names) != 1 || !isNamedReceiverType(
		pass.TypesInfo.TypeOf(function.Params.List[0].Type),
		"testing",
		"T",
	) {
		return nil
	}
	parameter, _ := pass.TypesInfo.Defs[function.Params.List[0].Names[0]].(*types.Var)
	return parameter
}

func hasTestingParallelCall(pass *Pass, body *ast.BlockStmt, parameter *types.Var) bool {
	found := false
	inspectFunctionBody(
		body,
		func(node ast.Node) bool {
			call,
				ok := node.(*ast.CallExpr)
			if !ok || !isTestingMethod(pass, call.Fun, "Parallel") {
				return true
			}
			selector,
				ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
			if !ok {
				return true
			}
			identifier,
				ok := ast.Unparen(selector.X).(*ast.Ident)
			if ok && pass.TypesInfo.ObjectOf(identifier) == parameter {
				found = true
			}
			return !found
		},
	)
	return found
}

func hasUnsafeParallelTestState(pass *Pass, body *ast.BlockStmt) bool {
	unsafe := false
	ast.Inspect(
		body,
		func(node ast.Node) bool {
			if unsafe || node == nil {
				return false
			}
			switch node := node.(type) {
			case *ast.CallExpr:
				if isTestingMethod(pass, node.Fun, "Setenv") || isTestingMethod(pass, node.Fun, "Chdir") || isUnsafeOSStateCall(pass, node.Fun) {
					unsafe = true
				}
			case *ast.AssignStmt:
				for _, left := range node.Lhs {
					if expressionMutatesPackageVariable(pass, left) {
						unsafe = true
						break
					}
				}
			case *ast.IncDecStmt:
				unsafe = expressionMutatesPackageVariable(pass, node.X)
			}
			return !unsafe
		},
	)
	return unsafe
}

func isTestingMethod(pass *Pass, expression ast.Expr, name string) bool {
	return isNamedMethod(pass.TypesInfo, expression, "testing", "T", name)
}

func isUnsafeOSStateCall(pass *Pass, expression ast.Expr) bool {
	function := calledFunction(pass.TypesInfo, expression)
	if function == nil || function.Pkg() == nil || function.Pkg().Path() != "os" {
		return false
	}
	switch function.Name() {
	case "Setenv", "Unsetenv", "Clearenv", "Chdir":
		return true
	default:
		return false
	}
}

func expressionMutatesPackageVariable(pass *Pass, expression ast.Expr) bool {
	object := rootExpressionObject(pass, expression)
	variable, ok := object.(*types.Var)
	return ok && variable.Pkg() != nil && variable.Parent() == variable.Pkg().Scope()
}

func rootExpressionObject(pass *Pass, expression ast.Expr) types.Object {
	switch expression := ast.Unparen(expression).(type) {
	case *ast.Ident:
		return pass.TypesInfo.ObjectOf(expression)
	case *ast.SelectorExpr:
		if object := pass.TypesInfo.ObjectOf(expression.Sel); object != nil {
			if variable, ok := object.(*types.Var); ok && variable.Pkg() != nil && variable.Parent() == variable.Pkg().Scope() {
				return variable
			}
		}
		return rootExpressionObject(pass, expression.X)
	case *ast.IndexExpr:
		return rootExpressionObject(pass, expression.X)
	case *ast.IndexListExpr:
		return rootExpressionObject(pass, expression.X)
	case *ast.StarExpr:
		return rootExpressionObject(pass, expression.X)
	default:
		return nil
	}
}

func (testParallelismCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
