package semantic

import "testing"

func TestIneffectiveBitwiseZeroReportsFixedResults(t *testing.T) {
	root := analysisModule(t, `package sample

const missingFlag = iota

func bits(value uint) uint {
	left := value & 0
	right := value ^ missingFlag
	return left | right
}
`)
	registry, err := newRegistry([]string{
		"ineffective-bitwise-zero",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnosticGolden(t, diagnostics)
}
