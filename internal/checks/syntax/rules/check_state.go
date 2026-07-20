package rules

import (
	"reflect"

	"github.com/gempir/strider/internal/cst"
)

type checkStateKey struct {
	code   string
	typeOf reflect.Type
}

type importCheckState struct {
	names map[string]bool
	paths map[string]bool
	seen  map[string]bool
}

type functionCheckState struct {
	receiverNames map[string]string
	marshalKinds  map[string]string
}

type namingCheckState struct {
	foldedNames map[string]map[string]string
}

type repeatedLiteralState struct {
	literals map[string][]*cst.BasicLit
}

type declarationCheckState struct {
	publicStructs int
}

func checkState[T any](pass *Pass, create func() *T) *T {
	key := checkStateKey{
		code:   pass.activeCode,
		typeOf: reflect.TypeFor[T](),
	}
	if current, ok := pass.checkState[key].(*T); ok {
		return current
	}
	current := create()
	pass.checkState[key] = current
	return current
}

func (a *Pass) imports() *importCheckState {
	return checkState(a, func() *importCheckState {
		return &importCheckState{
			names: make(map[string]bool),
			paths: make(map[string]bool),
			seen:  make(map[string]bool),
		}
	})
}

func (a *Pass) functionState() *functionCheckState {
	return checkState(a, func() *functionCheckState {
		return &functionCheckState{
			receiverNames: make(map[string]string),
			marshalKinds:  make(map[string]string),
		}
	})
}

func (a *Pass) namingState() *namingCheckState {
	return checkState(a, func() *namingCheckState {
		return &namingCheckState{
			foldedNames: make(map[string]map[string]string),
		}
	})
}

func (a *Pass) repeatedLiteralState() *repeatedLiteralState {
	return checkState(a, func() *repeatedLiteralState {
		return &repeatedLiteralState{
			literals: make(map[string][]*cst.BasicLit),
		}
	})
}

func (a *Pass) declarationState() *declarationCheckState {
	return checkState(a, func() *declarationCheckState {
		return &declarationCheckState{}
	})
}
