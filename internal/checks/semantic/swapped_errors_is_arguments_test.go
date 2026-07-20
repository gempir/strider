package semantic

import "testing"

func TestSwappedErrorsIsArgumentsReportsExternalSentinelFirst(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"errors"
	"io"
)

func match(err error) bool {
	bad := errors.Is(io.EOF, err)
	good := errors.Is(err, io.EOF)
	bothSentinels := errors.Is(io.EOF, errors.ErrUnsupported)
	return bad || good || bothSentinels
}
`,
	)
	registry, err := newRegistry([]string{
		"swapped-errors-is-arguments",
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
