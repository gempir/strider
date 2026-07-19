package semantic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestValidateOverlayTypeChecksFileQueryForInactiveSource(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.com/validateoverlay\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(directory, "active.go")
	inactive := filepath.Join(directory, "inactive.go")
	activeSource := []byte("package sample\n")
	inactiveSource := []byte("//go:build neverfix\n\npackage sample\n\nvar _ = missing\n")
	if err := os.WriteFile(active, activeSource, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inactive, inactiveSource, 0o600); err != nil {
		t.Fatal(err)
	}
	err := ValidateOverlay([]string{
		inactive,
	}, map[string][]byte{
		active:   activeSource,
		inactive: inactiveSource,
	})
	if err == nil {
		t.Fatal("invalid inactive source unexpectedly passed overlay validation")
	}
	if !strings.Contains(err.Error(), "undefined: missing") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestOverlayPresenceRequiresCompiledGoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.go")
	canonical, err := canonicalPath(path)
	if err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{
		canonical: true,
	}
	removeCompiledPaths(wanted, []*packages.Package{
		{
			GoFiles: []string{
				path,
			},
		},
	})
	if !wanted[canonical] {
		t.Fatal("GoFiles-only source was treated as type-checked")
	}
	removeCompiledPaths(wanted, []*packages.Package{
		{
			CompiledGoFiles: []string{
				path,
			},
		},
	})
	if wanted[canonical] {
		t.Fatal("compiled source was not recognized as type-checked")
	}
}
