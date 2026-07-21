package semantic

import "testing"

func TestInvalidListenAddressReportsInvalidConstants(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "net/http"

func serve(handler http.Handler, dynamic string) {
	http.ListenAndServe("localhost", handler)
	http.ListenAndServe(":70000", handler)
	http.ListenAndServe(":8080", handler)
	http.ListenAndServe(dynamic, handler)
}
`,
	)
	registry, err := newRegistry([]string{
		"invalid-listen-address",
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
