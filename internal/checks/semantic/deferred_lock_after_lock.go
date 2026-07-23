//strider:ignore-file cognitive-complexity,cyclomatic-complexity
package semantic

import (
	"go/ast"

	"github.com/gempir/strider/internal/diagnostic"
)

type deferredLockAfterLockCheck struct{}

func (deferredLockAfterLockCheck) Meta() Meta {
	return Meta{
		Code:            "deferred-lock-after-lock",
		Summary:         "detect deferring Lock immediately after locking",
		Explanation:     "Deferring Lock or RLock immediately after acquiring the same lock is almost always a typo for deferring Unlock or RUnlock and is likely to deadlock when the function returns.",
		GoodExample:     "mutex.Lock()\ndefer mutex.Unlock()",
		BadExample:      "mutex.Lock()\ndefer mutex.Lock()",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (deferredLockAfterLockCheck) Run(pass *Pass) {
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
				receiver, method, ok := syncLockExpression(pass, block.List[index])
				if !ok {
					continue
				}
				deferred, ok := block.List[index+1].(*ast.DeferStmt)
				if !ok {
					continue
				}
				deferredReceiver, deferredMethod, ok := syncLockCall(pass, deferred.Call)
				if !ok || method != deferredMethod || renderAnalysisExpression(pass, receiver) != renderAnalysisExpression(pass, deferredReceiver) {
					continue
				}
				unlock := "Unlock"
				if method == "RLock" {
					unlock = "RUnlock"
				}
				pass.Report(deferred, "defer "+unlock+" after locking; deferring "+method+" is likely a typo")
			}
			return true
		},
	)
}

func syncLockExpression(pass *Pass, statement ast.Stmt) (ast.Expr, string, bool) {
	expression, ok := statement.(*ast.ExprStmt)
	if !ok {
		return nil, "", false
	}
	call, ok := expression.X.(*ast.CallExpr)
	if !ok {
		return nil, "", false
	}
	return syncLockCall(pass, call)
}

func syncLockCall(pass *Pass, call *ast.CallExpr) (ast.Expr, string, bool) {
	if call == nil || len(call.Args) != 0 {
		return nil, "", false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (selector.Sel.Name != "Lock" && selector.Sel.Name != "RLock") {
		return nil, "", false
	}
	function, receiver := calledMethod(pass.TypesInfo, call.Fun)
	if function == nil || function.Pkg() == nil || function.Pkg().Path() != "sync" {
		return nil, "", false
	}
	if receiver == nil || receiver.Obj().Pkg() == nil || receiver.Obj().Pkg().Path() != "sync" || (receiver.Obj().Name() != "Mutex" && receiver.Obj().Name() != "RWMutex") {
		return nil, "", false
	}
	return selector.X, function.Name(), true
}
