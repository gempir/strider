package semantic

import "testing"

func TestDeferredLockAfterLockReportsRepeatedLock(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "sync"

func lock(mutex *sync.Mutex, rw *sync.RWMutex) {
	mutex.Lock()
	defer mutex.Lock()
	rw.RLock()
	defer rw.RUnlock()
}
`,
	)
	registry, err := newRegistry([]string{
		"deferred-lock-after-lock",
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
