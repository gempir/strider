package semantic

import "testing"

func TestAppendToSizedSliceConservativeCases(t *testing.T) {
	assertCheckDiagnostics(
		t,
		"append-to-sized-slice",
		`package sample

var packageValues = make([]int, 2)

func check() {
	values := make([]int, 3)
	values = append(values, 1)
	values = append(values, 2)
	loopValues := make([]int, 2)
	for value := range 2 {
		loopValues = append(loopValues, value)
	}

	reset := make([]int, 3)
	reset = reset[:0]
	reset = append(reset, 1)

	capacityOnly := make([]int, 0, 3)
	capacityOnly = append(capacityOnly, 1)
	packageValues = append(packageValues, 1)
}
`,
		2,
		"positive length",
	)
}
