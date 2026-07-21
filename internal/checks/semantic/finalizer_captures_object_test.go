package semantic

import "testing"

func TestFinalizerCapturesObjectReportsRetainedObject(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "runtime"

type resource struct{}

func leaking() {
	object := &resource{}
	runtime.SetFinalizer(object, func(*resource) {
		_ = object
	})
}

func clean() {
	object := &resource{}
	runtime.SetFinalizer(object, func(object *resource) {
		_ = object
	})
}
`,
	)
	registry, err := newRegistry([]string{
		"finalizer-captures-object",
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
