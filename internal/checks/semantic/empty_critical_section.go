package semantic

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type emptyCriticalSectionCheck struct{}

func (emptyCriticalSectionCheck) Meta() Meta {
	return Meta{
		Code:            "empty-critical-section",
		Summary:         "detect adjacent lock and unlock calls",
		Explanation:     "A lock immediately followed by its matching unlock protects no work and is commonly a missing defer. Intentional empty critical sections used for synchronization should be documented and suppressed explicitly.",
		GoodExample:     "mutex.Lock()\ndefer mutex.Unlock()",
		BadExample:      "mutex.Lock()\nmutex.Unlock()",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (emptyCriticalSectionCheck) Run(pass *Pass) {
	if pass.PackagePath == "sync_test" {
		return
	}
	pass.Inspect(
		[]ast.Node{
			(*ast.BlockStmt)(nil),
		},
		func(node ast.Node) bool {
			block, ok := node.(*ast.BlockStmt)
			if !ok || len(block.List) < 2 {
				return true
			}
			for index := 0; index+1 < len(block.List); index++ {
				firstReceiver, firstMethod, firstOK := lockStatement(pass, block.List[index])
				secondReceiver, secondMethod, secondOK := lockStatement(pass, block.List[index+1])
				if !firstOK || !secondOK || !matchingLockMethods(firstMethod, secondMethod) || renderAnalysisExpression(pass, firstReceiver) != renderAnalysisExpression(
					pass,
					secondReceiver,
				) {
					continue
				}
				pass.Report(block.List[index+1], "empty critical section; did you mean to defer the unlock?")
			}
			return true
		},
	)
}

func lockStatement(pass *Pass, statement ast.Stmt) (ast.Expr, string, bool) {
	expression, ok := statement.(*ast.ExprStmt)
	if !ok {
		return nil, "", false
	}
	call, ok := expression.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return nil, "", false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, "", false
	}
	function, ok := pass.TypesInfo.Uses[selector.Sel].(*types.Func)
	if !ok {
		return nil, "", false
	}
	signature, _ := function.Type().(*types.Signature)
	if signature == nil || signature.Params().Len() != 0 || signature.Results().Len() != 0 {
		return nil, "", false
	}
	return selector.X, function.Name(), true
}

func matchingLockMethods(lock, unlock string) bool {
	return lock == "Lock" && unlock == "Unlock" || lock == "RLock" && unlock == "RUnlock"
}

func renderAnalysisExpression(pass *Pass, expression ast.Expr) string {
	var output bytes.Buffer
	if err := format.Node(&output, pass.FileSet, expression); err != nil {
		return ""
	}
	return output.String()
}
