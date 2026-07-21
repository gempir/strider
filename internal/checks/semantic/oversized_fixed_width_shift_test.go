package semantic

import "testing"

func TestOversizedFixedWidthShiftReportsClearedValue(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func shifts(value uint8, machine uint) uint8 {
	cleared := value << 8
	value >>= 9
	_ = machine << 64
	_ = value << 7
	return cleared
}
`,
	)
	registry, err := newRegistry([]string{
		"oversized-fixed-width-shift",
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
