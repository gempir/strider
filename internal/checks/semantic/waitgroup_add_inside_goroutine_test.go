package semantic

import "testing"

func TestWaitGroupAddInsideGoroutineReportsRacyAdd(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "sync"

func start(group *sync.WaitGroup) {
	go func() {
		group.Add(1)
		defer group.Done()
	}()
	group.Add(1)
	go func() { defer group.Done() }()
}
`,
	)
	registry, err := newRegistry([]string{
		"waitgroup-add-inside-goroutine",
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
