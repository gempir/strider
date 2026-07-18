package analyze

import (
	"go/token"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type finalizerCapturesObjectRule struct {}

func (finalizerCapturesObjectRule) Meta() Meta {
	return Meta{
		Code: "finalizer-captures-object",
		Summary: "detect finalizers that retain the object they should release",
		Explanation: "A finalizer closure that captures the finalized object keeps that object reachable. The garbage collector can never make the object eligible for finalization, so the finalizer never runs and the object leaks. Use the finalizer function's parameter instead.",
		GoodExample: "runtime.SetFinalizer(object, func(object *resource) { object.Close() })",
		BadExample: "runtime.SetFinalizer(object, func(*resource) { object.Close() })",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (finalizerCapturesObjectRule) Run(pass *Pass) {
	for _, call := range pass.staticCallsInPackage("runtime") {
		if !isStaticFunction(call, "runtime", "SetFinalizer") || len(call.Common().Args) < 2 {
			continue
		}
		object := finalizerObject(call.Common().Args[0])
		closure, ok := unwrapSSAValue(call.Common().Args[1]).(*ssa.MakeClosure)
		if !ok || !closureCapturesFinalizerObject(closure, object) {
			continue
		}
		pass.Report(
			positionNode{position: call.Pos()},
			"finalizer captures the finalized object and prevents it from being collected; use the finalizer parameter instead",
		)
	}
}

func finalizerObject(value ssa.Value) ssa.Value {
	value = unwrapSSAValue(value)
	for {
		switch current := value.(type) {
		case *ssa.ChangeType:
			value = current.X
		case *ssa.Convert:
			value = current.X
		case *ssa.UnOp:
			if current.Op != token.MUL {
				return value
			}
			value = current.X
		default:
			return value
		}
	}
}

func closureCapturesFinalizerObject(closure *ssa.MakeClosure, object ssa.Value) bool {
	for _, binding := range closure.Bindings {
		if finalizerObject(binding) == object {
			return true
		}
	}
	return false
}
