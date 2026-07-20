package semantic

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type failedAssertionShadowReadCheck struct{}

func (failedAssertionShadowReadCheck) Meta() Meta {
	return Meta{
		Code:            "failed-assertion-shadow-read",
		Summary:         "detect reads of a shadowing failed type assertion result",
		Explanation:     "In an if initializer such as `if value, ok := value.(T); ok`, the new value variable is also in scope in the else branch. When the assertion fails it contains T's zero value, so reading it there usually means the original interface value was intended.",
		GoodExample:     "if typed, ok := value.(T); ok { use(typed) } else { logType(value) }",
		BadExample:      "if value, ok := value.(T); ok { use(value) } else { logType(value) }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (failedAssertionShadowReadCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.IfStmt)(nil),
		},
		func(node ast.Node) bool {
			statement, ok := node.(*ast.IfStmt)
			if !ok || statement.Else == nil {
				return true
			}
			shadow, ok := failedAssertionShadow(pass, statement)
			if !ok {
				return true
			}
			scanFailedAssertionStatement(pass, statement.Else, shadow, true)
			return true
		},
	)
}

func failedAssertionShadow(pass *Pass, statement *ast.IfStmt) (types.Object, bool) {
	assignment, ok := statement.Init.(*ast.AssignStmt)
	if !ok || assignment.Tok != token.DEFINE || len(assignment.Lhs) != 2 || len(assignment.Rhs) != 1 {
		return nil, false
	}
	shadow, ok := assignment.Lhs[0].(*ast.Ident)
	if !ok || shadow.Name == "_" {
		return nil, false
	}
	valid, ok := assignment.Lhs[1].(*ast.Ident)
	if !ok || valid.Name == "_" {
		return nil, false
	}
	assertion, ok := assignment.Rhs[0].(*ast.TypeAssertExpr)
	if !ok || assertion.Type == nil {
		return nil, false
	}
	original, ok := assertion.X.(*ast.Ident)
	if !ok || original.Name != shadow.Name {
		return nil, false
	}
	condition, ok := unparenExpression(statement.Cond).(*ast.Ident)
	if !ok || pass.TypesInfo.ObjectOf(condition) != pass.TypesInfo.ObjectOf(valid) {
		return nil, false
	}
	object := pass.TypesInfo.ObjectOf(shadow)
	if object == nil || object == pass.TypesInfo.ObjectOf(original) {
		return nil, false
	}
	return object, true
}

func unparenExpression(expression ast.Expr) ast.Expr {
	for {
		parenthesized, ok := expression.(*ast.ParenExpr)
		if !ok {
			return expression
		}
		expression = parenthesized.X
	}
}

// scanFailedAssertionStatement returns whether the failed assertion's zero
// value may still reach the end of statement. Direct assignments suppress
// diagnostics for later reads on paths where they definitely execute.
func scanFailedAssertionStatement(pass *Pass, statement ast.Stmt, shadow types.Object, active bool) bool {
	if statement == nil {
		return active
	}
	switch statement := statement.(type) {
	case *ast.BlockStmt:
		for _, child := range statement.List {
			active = scanFailedAssertionStatement(pass, child, shadow, active)
		}
		return active
	case *ast.AssignStmt:
		if active {
			for _, expression := range statement.Rhs {
				reportFailedAssertionReads(pass, expression, shadow)
			}
			for _, expression := range statement.Lhs {
				reportFailedAssertionLHSReads(pass, expression, shadow, statement.Tok != token.ASSIGN)
			}
		}
		return active && !assignsObject(pass, statement.Lhs, shadow)
	case *ast.IncDecStmt:
		if active {
			reportFailedAssertionReads(pass, statement.X, shadow)
		}
		return active && !expressionIsObject(statement.X, shadow, pass)
	case *ast.ExprStmt:
		if active {
			reportFailedAssertionReads(pass, statement.X, shadow)
		}
	case *ast.ReturnStmt:
		if active {
			for _, expression := range statement.Results {
				reportFailedAssertionReads(pass, expression, shadow)
			}
		}
	case *ast.DeferStmt:
		if active {
			reportFailedAssertionReads(pass, statement.Call, shadow)
		}
	case *ast.GoStmt:
		if active {
			reportFailedAssertionReads(pass, statement.Call, shadow)
		}
	case *ast.SendStmt:
		if active {
			reportFailedAssertionReads(pass, statement.Chan, shadow)
			reportFailedAssertionReads(pass, statement.Value, shadow)
		}
	case *ast.DeclStmt:
		if active {
			ast.Inspect(
				statement.Decl,
				func(node ast.Node) bool {
					if literal, ok := node.(*ast.FuncLit); ok && literal != nil {
						return false
					}
					identifier, ok := node.(*ast.Ident)
					if ok && pass.TypesInfo.ObjectOf(identifier) == shadow {
						pass.Report(identifier, failedAssertionMessage(identifier.Name))
					}
					return true
				},
			)
		}
	case *ast.IfStmt:
		if statement.Init != nil {
			active = scanFailedAssertionStatement(pass, statement.Init, shadow, active)
		}
		if active {
			reportFailedAssertionReads(pass, statement.Cond, shadow)
		}
		bodyActive := scanFailedAssertionStatement(pass, statement.Body, shadow, active)
		elseActive := active
		if statement.Else != nil {
			elseActive = scanFailedAssertionStatement(pass, statement.Else, shadow, active)
		}
		return bodyActive || elseActive
	case *ast.ForStmt:
		if statement.Init != nil {
			active = scanFailedAssertionStatement(pass, statement.Init, shadow, active)
		}
		if active && statement.Cond != nil {
			reportFailedAssertionReads(pass, statement.Cond, shadow)
		}
		scanFailedAssertionStatement(pass, statement.Body, shadow, active)
		if statement.Post != nil {
			scanFailedAssertionStatement(pass, statement.Post, shadow, active)
		}
		return active
	case *ast.RangeStmt:
		if active {
			reportFailedAssertionReads(pass, statement.X, shadow)
		}
		scanFailedAssertionStatement(pass, statement.Body, shadow, active)
		return active
	case *ast.LabeledStmt:
		return scanFailedAssertionStatement(pass, statement.Stmt, shadow, active)
	default:
		if active {
			reportFailedAssertionReads(pass, statement, shadow)
		}
	}
	return active
}

func reportFailedAssertionLHSReads(pass *Pass, expression ast.Expr, shadow types.Object, compound bool) {
	if identifier, ok := unparenExpression(expression).(*ast.Ident); ok {
		if compound && pass.TypesInfo.ObjectOf(identifier) == shadow {
			pass.Report(identifier, failedAssertionMessage(identifier.Name))
		}
		return
	}
	reportFailedAssertionReads(pass, expression, shadow)
}

func reportFailedAssertionReads(pass *Pass, node ast.Node, shadow types.Object) {
	ast.Inspect(
		node,
		func(node ast.Node) bool {
			if _, ok := node.(*ast.FuncLit); ok {
				return false
			}
			identifier, ok := node.(*ast.Ident)
			if ok && pass.TypesInfo.ObjectOf(identifier) == shadow {
				pass.Report(identifier, failedAssertionMessage(identifier.Name))
			}
			return true
		},
	)
}

func failedAssertionMessage(name string) string {
	return fmt.Sprintf("%s is the zero value produced by the failed type assertion, not the original interface value", name)
}

func assignsObject(pass *Pass, expressions []ast.Expr, object types.Object) bool {
	for _, expression := range expressions {
		identifier, ok := unparenExpression(expression).(*ast.Ident)
		if ok && pass.TypesInfo.ObjectOf(identifier) == object {
			return true
		}
	}
	return false
}

func expressionIsObject(expression ast.Expr, object types.Object, pass *Pass) bool {
	identifier, ok := unparenExpression(expression).(*ast.Ident)
	return ok && pass.TypesInfo.ObjectOf(identifier) == object
}

func (failedAssertionShadowReadCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
