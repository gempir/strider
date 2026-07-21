package semantic

import "testing"

func TestStandardHTTPMethodConstant(t *testing.T) {
	assertStyleCheck(t, "standard-http-method-constant")
}
