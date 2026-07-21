package semantic

import "testing"

func TestMisalignedAtomic64ReportsOn32BitTargets(t *testing.T) {
	t.Setenv("GOOS", "linux")
	t.Setenv("GOARCH", "386")
	root := analysisModule(
		t,
		`package sample

import "sync/atomic"

type counters struct {
	ready uint32
	total uint64
}

func add(value *counters) {
	atomic.AddUint64(&value.total, 1)
}
`,
	)
	registry, err := newRegistry([]string{
		"misaligned-atomic-64",
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
