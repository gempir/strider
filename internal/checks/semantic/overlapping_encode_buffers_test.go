package semantic

import "testing"

func TestOverlappingEncodeBuffersReportsSharedStarts(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"encoding/base64"
	"encoding/hex"
)

func encode(buffer, other []byte) {
	hex.Encode(buffer, buffer)
	base64.StdEncoding.Encode(buffer[:], buffer[:])
	hex.Encode(buffer, other)
	hex.Encode(buffer[1:], buffer[:1])
}
`,
	)
	registry, err := newRegistry([]string{
		"overlapping-encode-buffers",
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
