package semantic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeprecatedAPIUsageReportsDependencyMarkers(t *testing.T) {
	root := analysisModule(t, `package sample

import "example.com/analysis/legacy"

func use() int {
	legacy.Old()
	return legacy.Value{OldField: 1}.OldField
}
`)
	legacy := filepath.Join(root, "legacy")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(legacy, "legacy.go"),
		[]byte(`// Package legacy contains compatibility APIs.
//
// Deprecated: use the modern package instead.
package legacy

// Old should not be used.
//
// Deprecated: use New instead.
func Old() {}

type Value struct {
	// Deprecated: use NewField instead.
	OldField int
	NewField int
}
`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	registry, err := newRegistry([]string{
		"deprecated-api-usage",
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

func TestDeprecatedAPIUsageReportsImportedGenericAndInterfaceMethods(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "example.com/analysis/legacy"

func use(generic legacy.Generic[int], contract legacy.Contract) {
	generic.OldMethod()
	contract.OldInterfaceMethod()
}
`,
	)
	legacy := filepath.Join(root, "legacy")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(legacy, "legacy.go"),
		[]byte(`package legacy

type Generic[T any] struct{}

// Deprecated: use NewMethod instead.
func (Generic[T]) OldMethod() {}

type Contract interface {
	// Deprecated: use NewInterfaceMethod instead.
	OldInterfaceMethod()
}
`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	registry, err := newRegistry([]string{
		"deprecated-api-usage",
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

func TestDeprecatedAPIUsageReportsImportedGenericFields(t *testing.T) {
	root := analysisModule(t, `package sample

import "example.com/analysis/legacy"

func use(value legacy.Generic[int]) int {
	return value.OldField
}
`)
	legacy := filepath.Join(root, "legacy")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(legacy, "legacy.go"),
		[]byte(`package legacy

type Generic[T any] struct {
	// Deprecated: use NewField instead.
	OldField T
}
`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	registry, err := newRegistry([]string{
		"deprecated-api-usage",
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

func TestDeprecatedAPIUsageFollowsPhysicalFilesForLineDirectives(t *testing.T) {
	root := analysisModule(t, `package sample

import "example.com/analysis/legacy"

func use() {
	legacy.Old()
}
`)
	legacy := filepath.Join(root, "legacy")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "legacy.go"), []byte(`package legacy

// Deprecated: use New instead.
//line legacy.schema:400
func Old() {}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := newRegistry([]string{
		"deprecated-api-usage",
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

func TestDeprecatedAPIUsageReadsStandardLibraryMarkers(t *testing.T) {
	root := analysisModule(t, `package sample

import "io/ioutil"

func read() {
	_, _ = ioutil.ReadAll(nil)
}
`)
	registry, err := newRegistry([]string{
		"deprecated-api-usage",
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
