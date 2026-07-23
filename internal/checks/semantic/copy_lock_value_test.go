package semantic

import "testing"

func TestCopyLockValueReportsConservativeCopySites(t *testing.T) {
	reports := runCheckFixture(
		t,
		copyLockValueCheck{},
		`package fixture

import "sync"

type State struct {
	mu sync.Mutex
	value int
}

func receive(state State) {}
func (state State) valueMethod() {}
func pointer(state *State) {}

func copies(source State) State {
	target := source
	_ = target
	receive(source)
	output := make(chan State, 1)
	output <- source
	_ = struct { State State }{State: source}
	_ = map[State]int{source: 1}
	var states []State
	states = append(states, source)
	for _, item := range []State{source} {
		pointer(&item)
	}
	return source
}

func fresh() State { return State{} }
func good(source *State) { pointer(source) }
`,
	)
	assertReportCount(t, reports, 12)
	assertMessagesContain(t, reports, "sync.Mutex")
}
