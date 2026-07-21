package semantic

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInvalidRegexpReportsConstantInvalidRegexps(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import rx "regexp"

const invalid = "["

func check(pattern string) {
	rx.MustCompile(invalid)
	local := "("
	rx.Compile(local)
	rx.MatchString("[a-", "value")
	compile := rx.Compile
	compile("[")
	rx.Compile(pattern)
}
`,
	)
	registry, err := newRegistry([]string{
		"invalid-regexp",
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
		if item.Code != "invalid-regexp" || !strings.Contains(item.Message, "error parsing regexp") {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
		if item.Start.Filename != "main.go" && !(runtime.GOOS == "windows" && filepath.Base(filepath.FromSlash(item.Start.Filename)) == "main.go") {
			t.Fatalf("unexpected display path: %q", item.Start.Filename)
		}
	}
}

func TestInvalidRegexpAcceptsValidAndDynamicRegexps(t *testing.T) {
	root := analysisModule(t, `package sample

import "regexp"

func check(pattern string) {
	regexp.MustCompile("[a-z]+")
	regexp.Compile(pattern)
}
`)
	registry, err := newRegistry([]string{
		"invalid-regexp",
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
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
}
