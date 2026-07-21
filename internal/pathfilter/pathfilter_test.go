package pathfilter

import (
	"path/filepath"
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

func TestExcludedResolvesDiagnosticPathsFromRoot(t *testing.T) {
	root := t.TempDir()
	if !Excluded(root, "internal/generated.go", []string{
		"internal/**",
	}) {
		t.Fatal("root-relative diagnostic path did not match")
	}
}
