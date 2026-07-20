package semantic

import "testing"

func TestRedundantConversionConservativeCases(t *testing.T) {
	assertCheckDiagnostics(
		t,
		"redundant-conversion",
		`package sample

type UserID int

func check(existing UserID, raw int) {
	_ = UserID(existing)
	_ = UserID(raw)
}
`,
		1,
		"identical type",
	)
}
