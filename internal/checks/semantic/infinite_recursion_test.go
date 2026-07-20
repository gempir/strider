package semantic

import "testing"

func TestInfiniteRecursionReportsCallWithoutExitPath(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func infinite() {
	infinite()
}

func conditional(done bool) {
	if done {
		return
	}
	conditional(done)
}

func spawned() {
	go spawned()
}
`,
	)
	registry, err := newRegistry([]string{
		"infinite-recursion",
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
