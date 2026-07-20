package semantic

import "testing"

func TestByteStringWriteReportsAllocatingConversion(t *testing.T) {
	root := analysisModule(t, `package sample

import (
	"io"
	"os"
)

func write(bytes []byte) {
	io.WriteString(os.Stdout, string(bytes))
	os.Stdout.Write(bytes)
}
`)
	registry, err := newRegistry([]string{
		"byte-string-write",
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
