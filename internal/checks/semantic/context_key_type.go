package semantic

import (
	"fmt"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type contextKeyTypeRule struct{}

func (contextKeyTypeRule) Meta() Meta {
	return Meta{
		Code:            "context-key-type",
		Summary:         "detect unsafe context.WithValue key types",
		Explanation:     "Context keys must be comparable and should use a dedicated named type to avoid collisions between packages. Built-in types and anonymous empty structs risk collisions; non-comparable and nil keys panic at runtime.",
		GoodExample:     "type contextKey struct{}\nctx = context.WithValue(ctx, contextKey{}, value)",
		BadExample:      `ctx = context.WithValue(ctx, "request-id", value)`,
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (contextKeyTypeRule) Run(pass *Pass) {
	calls := pass.argumentsByCallPosition()
	for _, call := range pass.staticCallsInPackage("context") {
		if !isStaticFunction(call, "context", "WithValue") || len(call.Common().Args) <= 1 {
			continue
		}
		key := unwrapSSAValue(call.Common().Args[1])
		message := invalidContextKeyMessage(key)
		if message == "" {
			continue
		}
		node := explicitCallArgument(calls[call.Pos()], 1, call.Pos())
		pass.Report(node, message)
	}
}

func invalidContextKeyMessage(key ssa.Value) string {
	if isNilSSAConstant(key) {
		return "context.WithValue key must not be nil"
	}
	keyType := types.Unalias(key.Type())
	if _, ok := keyType.(*types.Basic); ok {
		return fmt.Sprintf("do not use built-in type %s as a context key; define a dedicated named type", types.TypeString(key.Type(), nil))
	}
	if structure, ok := keyType.(*types.Struct); ok && structure.NumFields() == 0 {
		return "do not use an anonymous empty struct as a context key; define a dedicated named type"
	}
	if !types.Comparable(keyType) {
		return fmt.Sprintf("context.WithValue key type %s is not comparable and will panic", types.TypeString(key.Type(), nil))
	}
	return ""
}
