package semantic

import "testing"

func TestZeroReplacementLimitReportsZeroConstants(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"bytes"
	"strings"
)

const none = 0

func replace(value string, raw []byte) {
	strings.Replace(value, "a", "b", none)
	bytes.Replace(raw, nil, nil, 0)
	strings.Replace(value, "a", "b", -1)
}
`,
	)
	registry, err := newRegistry([]string{
		"zero-replacement-limit",
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
