package semantic

import "testing"

func TestTestingFatalInGoroutineReportsChildTermination(t *testing.T) {
	root := analysisModule(t, `package sample

import "testing"

func TestWork(t *testing.T) {
	go func() { t.Fatal("failed") }()
	t.Log("running")
}
`)
	registry, err := newRegistry([]string{
		"testing-fatal-in-goroutine",
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
