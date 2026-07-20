package semantic

import "testing"

func TestExcessiveBlankIdentifiers(t *testing.T) {
	assertStyleCheck(t, "excessive-blank-identifiers")
}
