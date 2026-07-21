package semantic

import "testing"

func TestLeakyTimeTickReportsReturningFunctionsOnOlderGoVersions(t *testing.T) {
	root := analysisModuleVersion(
		t,
		"1.22",
		`package sample

import "time"

func returning() <-chan time.Time {
	return time.Tick(time.Second)
}

func endless() {
	for range time.Tick(time.Second) {}
}
`,
	)
	registry, err := newRegistry([]string{
		"leaky-time-tick",
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

func TestLeakyTimeTickAllowsModernGoVersions(t *testing.T) {
	root := analysisModule(t, `package sample

import "time"

func returning() <-chan time.Time {
	return time.Tick(time.Second)
}
`)
	registry, err := newRegistry([]string{
		"leaky-time-tick",
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
	if len(diagnostics) != 0 {
		t.Fatalf("got %d diagnostics, want none: %#v", len(diagnostics), diagnostics)
	}
}
