package semantic

import "testing"

func TestIPByteComparisonReportsTwoIPValues(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"bytes"
	"net"
)

func equal(left, right net.IP, raw []byte) bool {
	bad := bytes.Equal(left, right)
	_ = bytes.Equal(left, raw)
	good := left.Equal(right)
	return bad || good
}
`,
	)
	registry, err := newRegistry([]string{
		"ip-byte-comparison",
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
