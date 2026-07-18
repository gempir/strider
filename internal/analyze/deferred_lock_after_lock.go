package analyze

import (
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type deferredLockAfterLockRule struct {}

func (deferredLockAfterLockRule) Meta() Meta {
	return Meta{
		Code: "deferred-lock-after-lock",
		Summary: "detect deferring Lock immediately after locking",
		Explanation: "Deferring Lock or RLock immediately after acquiring the same lock is almost always a typo for deferring Unlock or RUnlock and is likely to deadlock when the function returns.",
		GoodExample: "mutex.Lock()\ndefer mutex.Unlock()",
		BadExample: "mutex.Lock()\ndefer mutex.Lock()",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (deferredLockAfterLockRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				block,
				ok := node.(*ast.BlockStmt)
				if !ok || len(block.List) < 2 {
					return true
				}
				for index := 0; index + 1 < len(block.List); index++ {
					receiver,
					method,
					ok := syncLockExpression(pass, block.List[index])
					if !ok {
						continue
					}
					deferred,
					ok := block.List[index + 1].(*ast.DeferStmt)
					if !ok {
						continue
					}
					deferredReceiver,
					deferredMethod,
					ok := syncLockCall(pass, deferred.Call)
					if !ok || method != deferredMethod || renderAnalysisExpression(pass, receiver) != renderAnalysisExpression(
						pass,
						deferredReceiver,
					) {
						continue
					}
					unlock := "Unlock"
					if method == "RLock" {
						unlock = "RUnlock"
					}
					pass.Report(
						deferred,
						"defer " + unlock + " after locking; deferring " + method + " is likely a typo",
					)
				}
				return true
			},
		)
	}
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
	function, ok := pass.TypesInfo.Uses[selector.Sel].(*types.Func)
	if !ok || function.Pkg() == nil || function.Pkg().Path() != "sync" {
		return nil, "", false
	}
	signature, _ := function.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		return nil, "", false
	}
	receiver := types.Unalias(signature.Recv().Type())
	if pointer, ok := receiver.(*types.Pointer); ok {
		receiver = types.Unalias(pointer.Elem())
	}
	named, ok := receiver.(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != "sync" || (named.Obj().Name() != "Mutex" && named.Obj().Name() != "RWMutex") {
		return nil, "", false
	}
	return selector.X, function.Name(), true
}
