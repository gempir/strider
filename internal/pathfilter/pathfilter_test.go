package pathfilter

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchesPrefixesAndDoublestarGlobs(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "internal", "generated", "file.go")
	for _, patterns := range [][]string{
		{
			"internal/generated",
		},
		{
			"internal/**/*.go",
		},
		{
			"**/generated/*.go",
		},
	} {
		if !Excluded(root, filename, patterns) {
			t.Errorf("%q did not match %q", filename, patterns)
		}
	}
	if Excluded(root, filename, []string{
		"cmd/**",
	}) {
		t.Fatal("unrelated glob matched")
	}
}

func TestValidateRejectsMalformedGlobsDeterministically(t *testing.T) {
	err := Validate([]string{
		"z/[",
		"a/{",
	})
	if err == nil {
		t.Fatal("malformed globs were accepted")
	}
	if got, want := err.Error(), `malformed exclusion glob(s): "a/{", "z/["`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if err := Validate([]string{
		"internal/**",
		"literal/path",
	}); err != nil {
		t.Fatalf("valid patterns were rejected: %v", err)
	}
	if !strings.Contains(err.Error(), "a/{") {
		t.Fatalf("error did not identify malformed pattern: %v", err)
	}
}

func TestExcludedResolvesDiagnosticPathsFromRoot(t *testing.T) {
	root := t.TempDir()
	if !Excluded(root, "internal/generated.go", []string{
		"internal/**",
	}) {
		t.Fatal("root-relative diagnostic path did not match")
	}
}
