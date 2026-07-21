package semantic

import "testing"

func TestDurationMultipliedByDurationReportsSquaredUnits(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "time"

func scale(duration time.Duration, count int) {
	_ = duration * time.Second
	_ = time.Duration(count) * time.Second
	_ = duration * 2
	_ = (1 + time.Duration(count)) * time.Millisecond
	_ = (duration / time.Second) * time.Second
	_ = (duration + time.Second) * time.Second
}
`,
	)
	registry, err := newRegistry([]string{
		"duration-multiplied-by-duration",
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
