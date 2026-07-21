package semantic

import "testing"

func TestNonCanonicalHeaderReportsNonCanonicalHeaderReads(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "net/http"

func check() {
	const key = "x-request-id"
	var request http.Request
	header := http.Header{}
	_ = header["content-type"]
	_ = header[key]
	_ = request.Header["etag"]
	_ = header["Content-Type"]
	header["content-type"] = nil
	request.Header["etag"] = nil
	header["Content-Type"] = request.Header["etag"]
	plain := map[string][]string{}
	_ = plain["content-type"]
}
`,
	)
	registry, err := newRegistry([]string{
		"non-canonical-header",
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
