package semantic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func TestDiscardedErrorResultTracksTypedResultPositions(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type writer interface { Write([]byte) (int, error) }

func pair() (int, error) { return 0, nil }
func errorFirst() (error, int) { return nil, 0 }
func one() error { return nil }
func noError() int { return 0 }

var _ = one()

func check(w writer) {
	pair()
	value, _ := pair()
	_, value = errorFirst()
	_ = one()
	_, value = one(), noError()
	w.Write(nil)
	noError()
	_, err := pair()
	err, _ = errorFirst()
	_ = value
	_ = err
}
`,
	)
	diagnostics := runStandaloneAnalysisRule(t, root, discardedErrorResultRule{})
	if len(diagnostics) != 7 {
		t.Fatalf("got %d diagnostics, want 7: %#v", len(diagnostics), diagnostics)
	}
	for _, item := range diagnostics {
		if item.Code != "discarded-error-result" || item.Severity != diagnostic.SeverityError {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}

func TestDiscardedErrorResultAllowsInfallibleWriters(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"bytes"
	"fmt"
	"strings"
)

func check() {
	var buffer bytes.Buffer
	var builder strings.Builder
	fmt.Fprintln(&buffer, "safe")
	buffer.WriteString("safe")
	builder.WriteString("safe")
}
`,
	)
	diagnostics := runStandaloneAnalysisRule(t, root, discardedErrorResultRule{})
	if len(diagnostics) != 0 {
		t.Fatalf("infallible writes reported: %#v", diagnostics)
	}
}

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
	diagnostics := runStandaloneAnalysisRule(t, root, testParallelismRule{})
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
	for _, item := range diagnostics {
		if item.Code != "test-parallelism" || item.Severity != diagnostic.SeverityNote {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}

func TestTopLevelDeclarationOrderReportsOneRegressionPerFile(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

const first = 1

var current item

type item struct{}

func init() { current = item{} }
func use() item { return current }

type later struct{}
const late = 2
`,
	)
	diagnostics := runStandaloneAnalysisRule(t, root, topLevelDeclarationOrderRule{})
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
	if diagnostics[0].Code != "top-level-declaration-order" || diagnostics[0].Severity != diagnostic.SeverityWarning {
		t.Fatalf("unexpected diagnostic: %#v", diagnostics[0])
	}
}

func TestTopLevelDeclarationOrderAcceptsConstVarTypeAndFunctions(t *testing.T) {
	root := analysisModule(t, `package sample

const first = 1

var current item

type item struct{}

func init() { current = item{} }
func use() item { return current }
`)
	diagnostics := runStandaloneAnalysisRule(t, root, topLevelDeclarationOrderRule{})
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
}

func TestAdditionalResearchRuleSeverities(t *testing.T) {
	tests := []struct {
		rule     Rule
		severity diagnostic.Severity
	}{
		{
			discardedErrorResultRule{},
			diagnostic.SeverityError,
		},
		{
			testParallelismRule{},
			diagnostic.SeverityNote,
		},
		{
			topLevelDeclarationOrderRule{},
			diagnostic.SeverityWarning,
		},
	}
	for _, test := range tests {
		if got := test.rule.Meta().DefaultSeverity; got != test.severity {
			t.Errorf("%s severity = %s, want %s", test.rule.Meta().Code, got, test.severity)
		}
	}
}

func runStandaloneAnalysisRule(t *testing.T, root string, rule Rule) []diagnostic.Diagnostic {
	t.Helper()
	meta := rule.Meta()
	registry := &Registry{
		rules: []Rule{
			rule,
		},
		settings: map[string]configuredRule{
			meta.Code: {
				severity: meta.DefaultSeverity,
			},
		},
		knownCodes: map[string]bool{
			meta.Code: true,
		},
	}
	diagnostics, err := Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	return diagnostics
}
