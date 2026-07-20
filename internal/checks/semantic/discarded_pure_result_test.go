package semantic

import "testing"

func TestDiscardedPureResultUsesOnlyKnownStandardFunctions(t *testing.T) {
	root := analysisModule(t, `package sample

import (
	"strings"
	"time"
)

func ignored(value string) {
	strings.TrimSpace(value)
	time.Now()
}
`)
	registry, err := newRegistry([]string{
		"discarded-pure-result",
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
