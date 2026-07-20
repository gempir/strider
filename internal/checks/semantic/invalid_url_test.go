package semantic

import (
	"strings"
	"testing"
)

func TestInvalidURLReportsInvalidConstantURLs(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "net/url"

func check(dynamic string) {
	url.Parse(":")
	invalid := "%gh&%"
	url.Parse(invalid)
	url.Parse("https://golang.org")
	url.Parse(dynamic)
}
`,
	)
	registry, err := newRegistry([]string{
		"invalid-url",
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
	for _, item := range diagnostics {
		if item.Code != "invalid-url" || !strings.Contains(item.Message, "not a valid URL") {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}
