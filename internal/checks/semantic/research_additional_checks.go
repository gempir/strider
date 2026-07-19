package semantic

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gempir/strider/internal/diagnostic"
)

type discardedErrorResultRule struct{}

func (discardedErrorResultRule) Meta() Meta {
	return Meta{
		Code:            "discarded-error-result",
		Summary:         "detect discarded error results from typed calls",
		Explanation:     "Ignoring an error result hides a failed operation from callers and can let execution continue with incomplete state. Handle or return actionable errors; conventional fmt output calls and writers whose contracts cannot return errors are excluded.",
		GoodExample:     "value, err := load(); if err != nil { return err }",
		BadExample:      "value, _ := load()",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (discardedErrorResultRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				switch node := node.(type) {
				case *ast.ExprStmt:
					call,
						ok := ast.Unparen(node.X).(*ast.CallExpr)
					if !ok || len(discardedErrorResultIndexes(pass, call)) == 0 {
						return true
					}
					reportDiscardedErrorResult(pass, call)
				case *ast.AssignStmt:
					reportBlankErrorResults(pass, node.Lhs, node.Rhs)
				case *ast.ValueSpec:
					left := make([]ast.Expr, len(node.Names))
					for index, name := range node.Names {
						left[index] = name
					}
					reportBlankErrorResults(pass, left, node.Values)
				}
				return true
			},
		)
	}
}

func reportBlankErrorResults(pass *Pass, left, right []ast.Expr) {
	if len(right) == 1 {
		call, ok := ast.Unparen(right[0]).(*ast.CallExpr)
		if !ok {
			return
		}
		for _, index := range discardedErrorResultIndexes(pass, call) {
			if index < len(left) && blankIdentifier(left[index]) {
				reportDiscardedErrorResult(pass, call)
				return
			}
		}
		return
	}
	if len(left) != len(right) {
		return
	}
	for index, expression := range right {
		if !blankIdentifier(left[index]) {
			continue
		}
		call, ok := ast.Unparen(expression).(*ast.CallExpr)
		if !ok {
			continue
		}
		results := discardedErrorResultIndexes(pass, call)
		if len(results) == 1 && results[0] == 0 && callResultCount(pass, call) == 1 {
			reportDiscardedErrorResult(pass, call)
		}
	}
}

func discardedErrorResultIndexes(pass *Pass, call *ast.CallExpr) []int {
	if discardedErrorResultIsInfallible(pass, call) {
		return nil
	}
	signature := callSignature(pass, call)
	if signature == nil {
		return nil
	}
	indexes := make([]int, 0, 1)
	for index := range signature.Results().Len() {
		if discardedResultIsError(signature.Results().At(index).Type()) {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func discardedErrorResultIsInfallible(pass *Pass, call *ast.CallExpr) bool {
	if isPackageFunction(pass.TypesInfo, call.Fun, "fmt", "Fprint") || isPackageFunction(pass.TypesInfo, call.Fun, "fmt", "Fprintf") || isPackageFunction(
		pass.TypesInfo,
		call.Fun,
		"fmt",
		"Fprintln",
	) {
		return true
	}
	if isPackageFunction(pass.TypesInfo, call.Fun, "io", "WriteString") {
		return len(call.Args) != 0 && infallibleWriterType(pass.TypesInfo.TypeOf(call.Args[0]))
	}
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	return ok && infallibleWriterType(pass.TypesInfo.TypeOf(selector.X))
}

func infallibleWriterType(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	valueType = types.Unalias(valueType)
	if pointer, ok := valueType.(*types.Pointer); ok {
		valueType = types.Unalias(pointer.Elem())
	}
	named, ok := valueType.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	packagePath := named.Obj().Pkg().Path()
	name := named.Obj().Name()
	return packagePath == "bytes" && name == "Buffer" || packagePath == "strings" && name == "Builder" || packagePath == "hash" && (name == "Hash" || name == "Hash32" || name == "Hash64")
}

func discardedResultIsError(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	return types.AssignableTo(valueType, types.Universe.Lookup("error").Type())
}

func callResultCount(pass *Pass, call *ast.CallExpr) int {
	signature := callSignature(pass, call)
	if signature == nil {
		return 0
	}
	return signature.Results().Len()
}

func callSignature(pass *Pass, call *ast.CallExpr) *types.Signature {
	valueType := pass.TypesInfo.TypeOf(call.Fun)
	if valueType == nil {
		return nil
	}
	signature, _ := types.Unalias(valueType).Underlying().(*types.Signature)
	return signature
}

func blankIdentifier(expression ast.Expr) bool {
	identifier, ok := ast.Unparen(expression).(*ast.Ident)
	return ok && identifier.Name == "_"
}

func reportDiscardedErrorResult(pass *Pass, call *ast.CallExpr) {
	name := renderAnalysisExpression(pass, call.Fun)
	if name == "" {
		name = "call"
	}
	pass.Report(call, fmt.Sprintf("error result returned by %s is discarded", name))
}

type inlineErrorDeclarationRule struct{}

func (inlineErrorDeclarationRule) Meta() Meta {
	return Meta{
		Code:            "inline-error-declaration",
		Summary:         "detect error variables declared in if and switch initializers",
		Explanation:     "Declaring an error in a control-statement initializer limits its scope and can encourage dense error handling. Declare the error immediately before the control statement when a longer-lived, easier-to-debug value is preferable.",
		GoodExample:     "value, err := load(); if err != nil { return err }",
		BadExample:      "if value, err := load(); err != nil { return err }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (inlineErrorDeclarationRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				var initializer ast.Stmt
				switch statement := node.(type) {
				case *ast.IfStmt:
					initializer = statement.Init
				case *ast.SwitchStmt:
					initializer = statement.Init
				case *ast.TypeSwitchStmt:
					initializer = statement.Init
				default:
					return true
				}
				assignment,
					ok := initializer.(*ast.AssignStmt)
				if !ok || assignment.Tok != token.DEFINE {
					return true
				}
				for _, expression := range assignment.Lhs {
					identifier,
						ok := ast.Unparen(expression).(*ast.Ident)
					if !ok || identifier.Name == "_" {
						continue
					}
					variable,
						_ := pass.TypesInfo.Defs[identifier].(*types.Var)
					if variable != nil && builtinErrorType(variable.Type()) {
						pass.Report(identifier, "declare the error before the control statement instead of in its initializer")
						break
					}
				}
				return true
			},
		)
	}
}

func builtinErrorType(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	return types.Identical(types.Unalias(valueType), types.Universe.Lookup("error").Type())
}

type testParallelismRule struct{}

func (testParallelismRule) Meta() Meta {
	return Meta{
		Code:            "test-parallelism",
		Summary:         "identify tests and direct subtests that can opt into parallel execution",
		Explanation:     "Independent tests can call t.Parallel to reduce suite latency. This advisory check skips tests that already opt in or visibly mutate process-global state, including environment and working-directory changes.",
		GoodExample:     "func TestLoad(t *testing.T) { t.Parallel(); checkLoad(t) }",
		BadExample:      "func TestLoad(t *testing.T) { checkLoad(t) }",
		DefaultSeverity: diagnostic.SeverityNote,
	}
}

func (testParallelismRule) Run(pass *Pass) {
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
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				call,
					ok := node.(*ast.CallExpr)
				if !ok || !isTestingMethod(pass, call.Fun, "Run") || len(call.Args) != 2 {
					return true
				}
				literal,
					ok := ast.Unparen(call.Args[1]).(*ast.FuncLit)
				if !ok {
					return true
				}
				parameter := testingTParameter(pass, literal.Type)
				if parameter == nil || literal.Body == nil || hasTestingParallelCall(pass, literal.Body, parameter) || hasUnsafeParallelTestState(
					pass,
					literal.Body,
				) {
					return true
				}
				pass.Report(literal, "consider calling t.Parallel() when this subtest begins")
				return true
			},
		)
	}
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
	if function == nil || function.Params == nil || len(function.Params.List) != 1 || len(function.Params.List[0].Names) != 1 || !isTestingTType(
		pass.TypesInfo.TypeOf(function.Params.List[0].Type),
	) {
		return nil
	}
	parameter, _ := pass.TypesInfo.Defs[function.Params.List[0].Names[0]].(*types.Var)
	return parameter
}

func isTestingTType(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	pointer, ok := types.Unalias(valueType).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(pointer.Elem()).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "testing" && named.Obj().Name() == "T"
}

func hasTestingParallelCall(pass *Pass, body *ast.BlockStmt, parameter *types.Var) bool {
	found := false
	inspectParallelTestBody(
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

func inspectParallelTestBody(body *ast.BlockStmt, visit func(ast.Node) bool) {
	first := true
	ast.Inspect(body, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		if _, nested := node.(*ast.FuncLit); nested && !first {
			return false
		}
		first = false
		return visit(node)
	})
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
	function := calledFunction(pass.TypesInfo, expression)
	if function == nil || function.Name() != name || function.Pkg() == nil || function.Pkg().Path() != "testing" {
		return false
	}
	signature, _ := function.Type().(*types.Signature)
	return signature != nil && signature.Recv() != nil && isTestingTType(signature.Recv().Type())
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

type declarationOrderRule struct{}

func (declarationOrderRule) Meta() Meta {
	return Meta{
		Code:            "declaration-order",
		Summary:         "keep top-level declarations in type, const, var, and func order",
		Explanation:     "A consistent top-level declaration order makes files easier to scan. Group types first, then constants, variables, and functions; imports are ignored and init remains in the function group.",
		GoodExample:     "type Client struct{}; const timeout = 1; var defaultClient Client; func New() Client { return Client{} }",
		BadExample:      "var defaultClient Client; type Client struct{}",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (declarationOrderRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		highest := -1
		for _, declaration := range file.Decls {
			rank := declarationKindRank(declaration)
			if rank < 0 {
				continue
			}
			if rank < highest {
				pass.Report(declaration, "top-level declarations should be ordered as type, const, var, then func")
				break
			}
			if rank > highest {
				highest = rank
			}
		}
	}
}

func declarationKindRank(declaration ast.Decl) int {
	switch declaration := declaration.(type) {
	case *ast.GenDecl:
		switch declaration.Tok {
		case token.IMPORT:
			return -1
		case token.TYPE:
			return 0
		case token.CONST:
			return 1
		case token.VAR:
			return 2
		default:
			return -1
		}
	case *ast.FuncDecl:
		return 3
	default:
		return -1
	}
}
