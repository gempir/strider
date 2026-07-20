package semantic

import (
	"fmt"
	"runtime"
	"testing"
)

func TestImpossiblePlatformComparisonReportsExcludedTarget(t *testing.T) {
	excluded := "windows"
	if runtime.GOOS == excluded {
		excluded = "linux"
	}
	root := analysisModule(t, fmt.Sprintf(`//go:build %s

package sample

import "runtime"

func impossible() bool {
	return runtime.GOOS == %q
}
`, runtime.GOOS, excluded))
	registry, err := newRegistry([]string{
		"impossible-platform-comparison",
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
