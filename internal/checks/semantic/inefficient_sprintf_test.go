package semantic

import "testing"

func TestInefficientSprintfConservativeCases(t *testing.T) {
	assertCheckDiagnostics(
		t,
		"inefficient-sprintf",
		`package sample

import "fmt"

type labeled string
type customInt int

func (l labeled) String() string { return "label:" + string(l) }
func (customInt) Format(fmt.State, rune) {}

func check(number int, text string, custom labeled) {
	_ = fmt.Sprintf("%d", number)
	_ = fmt.Sprintf("%s", text)
	_ = fmt.Sprintf("%v", number)
	_ = fmt.Sprintf("%f", float64(number))
	_ = fmt.Sprintf("%s", custom)
	_ = fmt.Sprintf("%d", customInt(number))
}
`,
		3,
		"unnecessary",
	)
}
