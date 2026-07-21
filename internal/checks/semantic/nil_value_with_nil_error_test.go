package semantic

import "testing"

func TestNilValueWithNilErrorReportsOnlyPayloadFunctions(t *testing.T) {
	reports := runResearchCorrectnessCheck(
		t,
		nilValueWithNilErrorCheck{},
		`package fixture

func badPointer() (*int, error) { return nil, nil }
func badSlice() ([]int, error) { return nil, nil }
func errorOnly() error { return nil }
func noErrorResult() (*int, bool) { return nil, false }
func good() (*int, error) { return nil, errMissing }

var errMissing = missingError{}
type missingError struct{}
func (missingError) Error() string { return "missing" }
`,
	)
	assertResearchReportCount(t, reports, 1)
	assertResearchMessagesContain(t, reports, "nil payload")
}
