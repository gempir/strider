package semantic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIdenticalBinaryOperandsReportsSuspiciousSelfComparison(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func compare(value int, floating float64) bool {
	bad := value == value
	allowedFloat := floating != floating
	return bad || allowedFloat
}
`,
	)
	registry, err := newRegistry([]string{
		"identical-binary-operands",
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

func TestCommandModuleEndingDotTestIsAnalyzed(t *testing.T) {
	root := analysisModule(t, `package main

func compare(value int) bool { return value == value }
`)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/analysis.test\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := newRegistry([]string{
		"identical-binary-operands",
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
