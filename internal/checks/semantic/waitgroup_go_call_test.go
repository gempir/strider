package semantic

import (
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func TestWaitGroupGoForbiddenCallUsesResolvedMethodsAndBuiltins(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "sync"

type fakeGroup struct{}

func (fakeGroup) Go(func()) {}
func (fakeGroup) Done() {}

func check(group *sync.WaitGroup, fake fakeGroup) {
	group.Go(func() {
		panic("failed")
		recover()
		group.Done()
		fake.Done()
	})
	fake.Go(func() {
		panic("allowed by this check")
		recover()
		group.Done()
	})
	group.Go(func() {
		panic := func(any) {}
		recover := func() any { return nil }
		panic("shadowed")
		_ = recover()
	})
	group.Go(func() {
		deferred := func() { group.Done() }
		_ = deferred
	})
}
`,
	)
	registry, err := newRegistry([]string{
		"waitgroup-go-forbidden-call",
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
	for _, item := range diagnostics {
		if item.Code != "waitgroup-go-forbidden-call" || item.Severity != diagnostic.SeverityError {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}
