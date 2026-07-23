//strider:ignore-file modifies-parameter
package syntax

import "github.com/gempir/strider/internal/cst"

type checkStates struct {
	imports       map[string]*importCheckState
	functions     map[string]*functionCheckState
	naming        map[string]*namingCheckState
	literals      map[string]*repeatedLiteralState
	declarations  map[string]*declarationCheckState
	documentation map[string]*documentationPeriodState
}

type importCheckState struct {
	names map[string]bool
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

func stateFor[T any](states *map[string]*T, code string, create func() *T) *T {
	if *states == nil {
		*states = make(map[string]*T)
	}
	if current := (*states)[code]; current != nil {
		return current
	}
	current := create()
	(*states)[code] = current
	return current
}

func (a *Pass) imports() *importCheckState {
	return stateFor(&a.states.imports, a.currentCode(), func() *importCheckState {
		return &importCheckState{
			names: make(map[string]bool),
			seen:  make(map[string]bool),
		}
	})
}

func (a *Pass) functionState() *functionCheckState {
	return stateFor(
		&a.states.functions,
		a.currentCode(),
		func() *functionCheckState {
			return &functionCheckState{
				receiverNames: make(map[string]string),
				marshalKinds:  make(map[string]string),
			}
		},
	)
}

func (a *Pass) namingState() *namingCheckState {
	return stateFor(&a.states.naming, a.currentCode(), func() *namingCheckState {
		return &namingCheckState{
			foldedNames: make(map[string]map[string]string),
		}
	})
}

func (a *Pass) repeatedLiteralState() *repeatedLiteralState {
	return stateFor(&a.states.literals, a.currentCode(), func() *repeatedLiteralState {
		return &repeatedLiteralState{
			literals: make(map[string][]*cst.BasicLit),
		}
	})
}

func (a *Pass) declarationState() *declarationCheckState {
	return stateFor(&a.states.declarations, a.currentCode(), func() *declarationCheckState {
		return &declarationCheckState{}
	})
}
