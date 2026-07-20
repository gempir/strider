package semantic

import "testing"

func TestDangerousDirectoryRemovalReportsWholeDirectory(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"os"
	"path/filepath"
)

func remove() {
	temporary := os.TempDir()
	os.RemoveAll(temporary)
	home, _ := os.UserHomeDir()
	os.RemoveAll(home)
	os.RemoveAll(filepath.Join(temporary, "application"))
}
`,
	)
	registry, err := newRegistry([]string{
		"dangerous-directory-removal",
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
