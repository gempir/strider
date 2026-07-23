package semantic

import "testing"

func TestNilErrorReturnReportsContradictoryReturns(t *testing.T) {
	reports := runCheckFixture(
		t,
		nilErrorReturnCheck{},
		`package fixture

func bad(err error) (*int, error) {
	if err != nil {
		return nil, nil
	}
	return new(int), nil
}

func badElse(err error) (*int, error) {
	if err == nil {
		return new(int), nil
	} else {
		return nil, nil
	}
}

func good(err error) (*int, error) {
	if err != nil {
		return nil, err
	}
	return new(int), nil
}

func reassigned(err error) (*int, error) {
	if err != nil {
		err = nil
		return nil, nil
	}
	return new(int), nil
}

func reassignedAfterBadReturn(err error, cond bool) (*int, error) {
	if err != nil {
		if cond {
			return nil, nil
		}
		err = nil
		return nil, nil
	}
	return new(int), nil
}
`,
	)
	assertReportCount(t, reports, 3)
	assertMessagesContain(t, reports, "proves an error is non-nil")
}
