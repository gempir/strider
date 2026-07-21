package semantic

import (
	"strings"
	"testing"
)

func TestSuspiciousSleepReportsSmallBareSleepLiterals(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "time"

const one = 1

func check() {
	time.Sleep(1)
	time.Sleep(42)
	time.Sleep(0)
	time.Sleep(121)
	time.Sleep(one)
	time.Sleep(2 * time.Nanosecond)
}
`,
	)
	registry, err := newRegistry([]string{
		"suspicious-sleep",
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
		if item.Code != "suspicious-sleep" || !strings.Contains(item.Message, "nanoseconds") {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}
