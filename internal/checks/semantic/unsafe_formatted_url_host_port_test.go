package semantic

import "testing"

func TestUnsafeFormattedURLHostPortReportsIPv6UnsafeFormat(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"fmt"
	"net"
	"strconv"
)

func urls(host string, port int) {
	_ = fmt.Sprintf("http://%s:%d/path", host, port)
	_, _ = net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	_ = fmt.Sprintf("%s:%d", host, port)
	_ = fmt.Sprintf("%s:%d", "file.go", 42)
	_ = "http://" + net.JoinHostPort(host, strconv.Itoa(port))
}
`,
	)
	registry, err := newRegistry([]string{
		"unsafe-formatted-url-host-port",
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
