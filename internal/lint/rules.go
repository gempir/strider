package lint

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/gempir/strider/internal/diagnostic"
)

const warning = diagnostic.SeverityWarning

type complexityRule struct{}

func (complexityRule) Meta() RuleMeta {
	return RuleMeta{
		Code: "cyclomatic-complexity", Summary: "limit branching complexity", DefaultSeverity: warning,
		Explanation: "Functions with too many independent control-flow paths are difficult to understand and test. The spike limit is 10.",
		GoodExample: "func sign(n int) int { if n < 0 { return -1 }; return 1 }",
		BadExample:  "func tangled() { /* more than ten branches */ }",
	}
}
func (complexityRule) Nodes() []ast.Node { return []ast.Node{(*ast.FuncDecl)(nil)} }
func (rule complexityRule) Run(context *Context, node ast.Node) {
	function := node.(*ast.FuncDecl)
	if function.Body == nil {
		return
	}
	complexity := 1
	ast.Inspect(function.Body, func(child ast.Node) bool {
		switch current := child.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.TypeSwitchStmt:
			complexity++
		case *ast.CaseClause:
			if len(current.List) != 0 {
				complexity++
			}
		case *ast.CommClause:
			if current.Comm != nil {
				complexity++
			}
		case *ast.BinaryExpr:
			if current.Op == token.LAND || current.Op == token.LOR {
				complexity++
			}
		}
		return true
	})
	if complexity > 10 {
		context.Report(function.Name, rule.Meta().Code, fmt.Sprintf("function complexity is %d; maximum is 10", complexity), warning)
	}
}

type maxParametersRule struct{}

func (maxParametersRule) Meta() RuleMeta {
	return RuleMeta{
		Code: "max-parameters", Summary: "limit function parameter count", DefaultSeverity: warning,
		Explanation: "Functions with more than five parameters tend to hide missing domain objects and are difficult to call correctly.",
		GoodExample: "func Open(path string, flags Flags) error",
		BadExample:  "func Open(path string, read, write, create, truncate, appendMode bool) error",
	}
}
func (maxParametersRule) Nodes() []ast.Node { return []ast.Node{(*ast.FuncDecl)(nil)} }
func (rule maxParametersRule) Run(context *Context, node ast.Node) {
	function := node.(*ast.FuncDecl)
	count := fieldCount(function.Type.Params)
	if count > 5 {
		context.Report(function.Name, rule.Meta().Code, fmt.Sprintf("function has %d parameters; maximum is 5", count), warning)
	}
}

func fieldCount(list *ast.FieldList) int {
	if list == nil {
		return 0
	}
	count := 0
	for _, field := range list.List {
		if len(field.Names) == 0 {
			count++
		} else {
			count += len(field.Names)
		}
	}
	return count
}

type nakedReturnRule struct{}

func (nakedReturnRule) Meta() RuleMeta {
	return RuleMeta{
		Code: "no-naked-return", Summary: "require explicit return values", DefaultSeverity: warning,
		Explanation: "A bare return in a function with named results makes data flow implicit, especially in longer functions.",
		GoodExample: "func value() (n int) { n = 1; return n }",
		BadExample:  "func value() (n int) { n = 1; return }",
	}
}
func (nakedReturnRule) Nodes() []ast.Node { return []ast.Node{(*ast.ReturnStmt)(nil)} }
func (rule nakedReturnRule) Run(context *Context, node ast.Node) {
	statement := node.(*ast.ReturnStmt)
	if len(statement.Results) != 0 || !enclosingFunctionHasNamedResults(context.Ancestors()) {
		return
	}
	context.Report(statement, rule.Meta().Code, "return values must be explicit", warning)
}

func enclosingFunctionHasNamedResults(ancestors []ast.Node) bool {
	for index := len(ancestors) - 1; index >= 0; index-- {
		var results *ast.FieldList
		switch function := ancestors[index].(type) {
		case *ast.FuncDecl:
			results = function.Type.Results
		case *ast.FuncLit:
			results = function.Type.Results
		default:
			continue
		}
		if results == nil {
			return false
		}
		for _, field := range results.List {
			if len(field.Names) != 0 {
				return true
			}
		}
		return false
	}
	return false
}

type noInitRule struct{}

func (noInitRule) Meta() RuleMeta {
	return RuleMeta{
		Code: "no-init", Summary: "avoid implicit package initialization", DefaultSeverity: warning,
		Explanation: "init functions hide ordering and side effects. Prefer explicit construction called from main or tests.",
		GoodExample: "func Configure() error { return register() }",
		BadExample:  "func init() { register() }",
	}
}
func (noInitRule) Nodes() []ast.Node { return []ast.Node{(*ast.FuncDecl)(nil)} }
func (rule noInitRule) Run(context *Context, node ast.Node) {
	function := node.(*ast.FuncDecl)
	if function.Recv == nil && function.Name.Name == "init" {
		context.Report(function.Name, rule.Meta().Code, "replace init with explicit initialization", warning)
	}
}

type noPackageVarRule struct{}

func (noPackageVarRule) Meta() RuleMeta {
	return RuleMeta{
		Code: "no-package-var", Summary: "avoid mutable package state", DefaultSeverity: warning,
		Explanation: "Package variables create shared mutable state and make dependencies, tests, and concurrency harder to reason about. Blank-identifier compile-time assertions are exempt.",
		GoodExample: "const defaultLimit = 10",
		BadExample:  "var defaultClient = newClient()",
	}
}
func (noPackageVarRule) Nodes() []ast.Node { return []ast.Node{(*ast.GenDecl)(nil)} }
func (rule noPackageVarRule) Run(context *Context, node ast.Node) {
	declaration := node.(*ast.GenDecl)
	if declaration.Tok != token.VAR {
		return
	}
	if _, ok := context.Parent().(*ast.File); !ok {
		return
	}
	for _, specNode := range declaration.Specs {
		spec := specNode.(*ast.ValueSpec)
		for _, name := range spec.Names {
			if name.Name != "_" {
				context.Report(name, rule.Meta().Code, "package variables introduce mutable global state", warning)
			}
		}
	}
}

type noDeferInLoopRule struct{}

func (noDeferInLoopRule) Meta() RuleMeta {
	return RuleMeta{
		Code: "no-defer-in-loop", Summary: "avoid accumulating defers in loops", DefaultSeverity: warning,
		Explanation: "A defer runs when the surrounding function returns, not when an iteration ends, so resources can accumulate unexpectedly.",
		GoodExample: "for rows.Next() { if err := handleRow(rows); err != nil { return err } }",
		BadExample:  "for rows.Next() { defer rows.Close() }",
	}
}
func (noDeferInLoopRule) Nodes() []ast.Node { return []ast.Node{(*ast.DeferStmt)(nil)} }
func (rule noDeferInLoopRule) Run(context *Context, node ast.Node) {
	ancestors := context.Ancestors()
	for index := len(ancestors) - 1; index >= 0; index-- {
		switch ancestors[index].(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return
		case *ast.ForStmt, *ast.RangeStmt:
			context.Report(node, rule.Meta().Code, "defer inside a loop runs at function exit, not iteration exit", warning)
			return
		}
	}
}

type noElseAfterReturnRule struct{}

func (noElseAfterReturnRule) Meta() RuleMeta {
	return RuleMeta{
		Code: "no-else-after-return", Summary: "remove else after terminal return", DefaultSeverity: warning,
		Explanation: "When the if branch returns, the else branch can be unindented. This reduces nesting without changing control flow.",
		GoodExample: "if err != nil { return err }\nuse(value)",
		BadExample:  "if err != nil { return err } else { use(value) }",
	}
}
func (noElseAfterReturnRule) Nodes() []ast.Node { return []ast.Node{(*ast.IfStmt)(nil)} }
func (rule noElseAfterReturnRule) Run(context *Context, node ast.Node) {
	statement := node.(*ast.IfStmt)
	if statement.Else == nil || len(statement.Body.List) == 0 {
		return
	}
	if _, ok := statement.Body.List[len(statement.Body.List)-1].(*ast.ReturnStmt); ok {
		context.Report(statement.Else, rule.Meta().Code, "remove else and unindent its body after the return", warning)
	}
}
