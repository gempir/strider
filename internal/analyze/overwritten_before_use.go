package analyze

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type overwrittenBeforeUseRule struct {}

func (overwrittenBeforeUseRule) Meta() Meta {
	return Meta{
		Code: "overwritten-before-use",
		Summary: "detect assigned values that are replaced before being used",
		Explanation: "A non-constant value assigned to a local variable but never read is often a forgotten error check or dead computation. The analyzer follows SSA phi nodes so values flowing across control-flow joins still count as used.",
		GoodExample: "result := calculate(); use(result); result = calculateAgain()",
		BadExample: "result := calculate(); result = calculateAgain()",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (overwrittenBeforeUseRule) Run(pass *Pass) {
	for _, function := range pass.Functions {
		if function == nil || function.Synthetic != "" || function.Blocks == nil || isExampleFunction(
			function,
		) {
			continue
		}
		syntax := function.Syntax()
		if syntax == nil {
			continue
		}
		switchTags := functionSwitchTags(function, syntax)
		inspectFunctionSyntax(
			syntax,
			func(node ast.Node) bool {
				switch node := node.(type) {
				case *ast.IncDecStmt:
					reportUnusedIncrementValue(pass, function, node, switchTags)
				case *ast.AssignStmt:
					reportUnusedAssignedValues(pass, function, node, switchTags)
				}
				return true
			},
		)
	}
}

func isExampleFunction(function *ssa.Function) bool {
	if !strings.HasPrefix(function.Name(), "Example") {
		return false
	}
	signature := function.Signature
	return signature != nil && signature.Recv() == nil && signature.Params().Len() == 0 && signature.Results().Len() == 0
}

func inspectFunctionSyntax(root ast.Node, visit func(ast.Node) bool) {
	first := true
	ast.Inspect(
		root,
		func(node ast.Node) bool {
			if _,
			ok := node.(*ast.FuncLit); ok && !first {
				return false
			}
			first = false
			return visit(node)
		},
	)
}

func functionSwitchTags(function *ssa.Function, syntax ast.Node) map[ssa.Value]bool {
	tags := make(map[ssa.Value]bool)
	inspectFunctionSyntax(
		syntax,
		func(node ast.Node) bool {
			switchStatement,
			ok := node.(*ast.SwitchStmt)
			if !ok || switchStatement.Tag == nil {
				return true
			}
			value,
			_ := function.ValueForExpr(switchStatement.Tag)
			if value != nil {
				tags[value] = true
			}
			return true
		},
	)
	return tags
}

func reportUnusedIncrementValue(
	pass *Pass,
	function *ssa.Function,
	statement *ast.IncDecStmt,
	switchTags map[ssa.Value]bool,
) {
	value, _ := function.ValueForExpr(statement.X)
	if value == nil {
		return
	}
	if _, constant := value.(*ssa.Const); constant || ssaValueHasUse(value, switchTags, nil) {
		return
	}
	pass.Report(
		statement,
		fmt.Sprintf(
			"this value of %s is overwritten before use",
			renderAnalysisExpression(pass, statement.X),
		),
	)
}

func reportUnusedAssignedValues(
	pass *Pass,
	function *ssa.Function,
	statement *ast.AssignStmt,
	switchTags map[ssa.Value]bool,
) {
	if len(statement.Lhs) > 1 && len(statement.Rhs) == 1 {
		reportUnusedTupleValues(pass, function, statement, switchTags)
		return
	}
	for index, left := range statement.Lhs {
		identifier, ok := left.(*ast.Ident)
		if !ok || identifier.Name == "_" || index >= len(statement.Rhs) {
			continue
		}
		value, _ := function.ValueForExpr(statement.Rhs[index])
		if value == nil && statement.Tok != token.ASSIGN {
			value, _ = function.ValueForExpr(left)
		}
		if value == nil {
			continue
		}
		if _, constant := value.(*ssa.Const); constant || ssaValueHasUse(value, switchTags, nil) {
			continue
		}
		pass.Report(
			statement,
			fmt.Sprintf("this value of %s is overwritten before use", identifier.Name),
		)
	}
}

func reportUnusedTupleValues(
	pass *Pass,
	function *ssa.Function,
	statement *ast.AssignStmt,
	switchTags map[ssa.Value]bool,
) {
	tuple, _ := function.ValueForExpr(statement.Rhs[0])
	if tuple == nil || tuple.Referrers() == nil {
		return
	}
	for _, reference := range *tuple.Referrers() {
		extract, ok := reference.(*ssa.Extract)
		if !ok || extract.Index >= len(statement.Lhs) || ssaValueHasUse(extract, switchTags, nil) {
			continue
		}
		identifier, ok := statement.Lhs[extract.Index].(*ast.Ident)
		if !ok || identifier.Name == "_" {
			continue
		}
		pass.Report(
			statement,
			fmt.Sprintf("this value of %s is overwritten before use", identifier.Name),
		)
	}
}

func ssaValueHasUse(value ssa.Value, switchTags map[ssa.Value]bool, seen map[ssa.Value]bool) bool {
	if switchTags[value] {
		return true
	}
	if seen[value] {
		return false
	}
	references := value.Referrers()
	if references == nil {
		return true
	}
	for _, reference := range *references {
		switch reference := reference.(type) {
		case *ssa.DebugRef:
			continue
		case *ssa.Phi:
			if seen == nil {
				seen = make(map[ssa.Value]bool)
			}
			seen[value] = true
			if ssaValueHasUse(reference, switchTags, seen) {
				return true
			}
		default:
			return true
		}
	}
	return false
}
