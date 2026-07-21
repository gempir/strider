package semantic

import "testing"

func TestEmptyCriticalSectionReportsAdjacentUnlock(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "sync"

func lock(mutex *sync.RWMutex) {
	mutex.Lock()
	mutex.Unlock()
	mutex.RLock()
	use()
	mutex.RUnlock()
}

func use() {}
`,
	)
	registry, err := newRegistry([]string{
		"empty-critical-section",
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
