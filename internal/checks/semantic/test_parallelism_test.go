package semantic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func TestTestParallelismReportsOnlyPlausiblyIndependentTests(t *testing.T) {
	root := analysisModule(t, `package sample

import "testing"

var shared int

func helper() {}
func TestProduction(t *testing.T) {}
`)
	if err := os.WriteFile(
		filepath.Join(root, "parallel_test.go"),
		[]byte(`package sample

import "testing"

func TestEligible(t *testing.T) {
	helper()
}

func TestAlreadyParallel(t *testing.T) {
	t.Parallel()
	helper()
}

func TestEnvironment(t *testing.T) {
	t.Setenv("STRIDER_TEST_KEY", "value")
}

func TestGlobalMutation(t *testing.T) {
	shared++
}

func Testhelper(t *testing.T) {}

func TestSubtests(t *testing.T) {
	t.Run("eligible", func(t *testing.T) {
		helper()
	})
	t.Run("parallel", func(t *testing.T) {
		t.Parallel()
	})
	t.Run("environment", func(t *testing.T) {
		t.Setenv("STRIDER_SUBTEST_KEY", "value")
	})
}
`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	diagnostics := runStandaloneAnalysisCheck(t, root, testParallelismCheck{})
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
	for _, item := range diagnostics {
		if item.Code != "test-parallelism" || item.Severity != diagnostic.SeverityNote {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}
