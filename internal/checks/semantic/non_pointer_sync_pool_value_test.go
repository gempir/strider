package semantic

import "testing"

func TestNonPointerSyncPoolValueReportsBoxedValues(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "sync"

func store(pool *sync.Pool) {
	bytes := []byte("value")
	pool.Put(bytes)
	pool.Put(42)
	pool.Put(&bytes)
	pool.Put(map[string]int{})
}
`,
	)
	registry, err := newRegistry([]string{
		"non-pointer-sync-pool-value",
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
