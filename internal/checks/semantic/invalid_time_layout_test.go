package semantic

import (
	"strings"
	"testing"
)

func TestInvalidTimeLayoutReportsInvalidConstantTimeLayouts(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "time"

const invalidLayout = "12345"

func check(value string) {
	time.Parse(invalidLayout, value)
	local := "12345"
	time.Parse(local, value)
	time.Parse("2006-01-02", value)
	time.Parse(time.RFC3339Nano, value)
}
`,
	)
	registry, err := newRegistry([]string{
		"invalid-time-layout",
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
		if item.Code != "invalid-time-layout" || !strings.Contains(item.Message, "parsing time") {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}
