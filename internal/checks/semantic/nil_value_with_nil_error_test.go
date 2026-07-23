package semantic

import "testing"

func TestNilValueWithNilErrorReportsOnlyPayloadFunctions(t *testing.T) {
	reports := runCheckFixture(
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
	assertReportCount(t, reports, 1)
	assertMessagesContain(t, reports, "nil payload")
}
