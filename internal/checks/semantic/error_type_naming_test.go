package semantic

import "testing"

func TestErrorTypeNaming(t *testing.T) {
	assertStyleCheck(t, "error-type-naming")
}
