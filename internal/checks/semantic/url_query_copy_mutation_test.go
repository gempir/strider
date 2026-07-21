package semantic

import "testing"

func TestURLQueryCopyMutationReportsTemporaryMapChange(t *testing.T) {
	root := analysisModule(t, `package sample

import "net/url"

func update(address *url.URL) {
	address.Query().Set("mode", "fast")
}
`)
	registry, err := newRegistry([]string{
		"url-query-copy-mutation",
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
