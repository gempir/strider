package semantic

import "testing"

func TestTimerResetDrainRaceReportsConditionalDrain(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "time"

func reset(timer *time.Timer, delay time.Duration) {
	if !timer.Reset(delay) {
		<-timer.C
	}
	timer.Reset(delay)
}
`,
	)
	registry, err := newRegistry([]string{
		"timer-reset-drain-race",
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
