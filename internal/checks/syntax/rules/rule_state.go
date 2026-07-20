package rules

import (
	"reflect"

	"github.com/gempir/strider/internal/cst"
)

type ruleStateKey struct {
	code   string
	typeOf reflect.Type
}

type importRuleState struct {
	names map[string]bool
	paths map[string]bool
	seen  map[string]bool
}

type functionRuleState struct {
	receiverNames map[string]string
	marshalKinds  map[string]string
}

type namingRuleState struct {
	foldedNames map[string]map[string]string
}

type repeatedLiteralState struct {
	literals map[string][]*cst.BasicLit
}

type declarationRuleState struct {
	publicStructs int
}

func ruleState[T any](pass *Pass, create func() *T) *T {
	key := ruleStateKey{
		code:   pass.activeCode,
		typeOf: reflect.TypeFor[T](),
	}
	if current, ok := pass.ruleState[key].(*T); ok {
		return current
	}
	current := create()
	pass.ruleState[key] = current
	return current
}

func (a *cstAnalyzer) imports() *importRuleState {
	return ruleState(a, func() *importRuleState {
		return &importRuleState{
			names: make(map[string]bool),
			paths: make(map[string]bool),
			seen:  make(map[string]bool),
		}
	})
}

func (a *cstAnalyzer) functionState() *functionRuleState {
	return ruleState(a, func() *functionRuleState {
		return &functionRuleState{
			receiverNames: make(map[string]string),
			marshalKinds:  make(map[string]string),
		}
	})
}

func (a *cstAnalyzer) namingState() *namingRuleState {
	return ruleState(a, func() *namingRuleState {
		return &namingRuleState{
			foldedNames: make(map[string]map[string]string),
		}
	})
}

func (a *cstAnalyzer) repeatedLiteralState() *repeatedLiteralState {
	return ruleState(a, func() *repeatedLiteralState {
		return &repeatedLiteralState{
			literals: make(map[string][]*cst.BasicLit),
		}
	})
}

func (a *cstAnalyzer) declarationState() *declarationRuleState {
	return ruleState(a, func() *declarationRuleState {
		return &declarationRuleState{}
	})
}
