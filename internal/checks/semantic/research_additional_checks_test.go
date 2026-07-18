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

func TestInlineErrorDeclarationOnlyReportsBuiltinErrors(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type ParseError struct{}
func (ParseError) Error() string { return "parse" }

type ErrorAlias = error

func pair() (int, error) { return 0, nil }
func one() error { return nil }
func aliasError() ErrorAlias { return nil }
func concreteError() ParseError { return ParseError{} }

func check() {
	if value, err := pair(); err != nil {
		_ = value
	}
	switch err := one(); err {
	case nil:
	}
	switch err := one(); value := any(1).(type) {
	case int:
		_ = err
		_ = value
	}
	if err := aliasError(); err != nil {
		_ = err
	}
	if err := concreteError(); err.Error() != "" {
		_ = err
	}
	var err error
	if err = one(); err != nil {
		_ = err
	}
}
`,
	)
	diagnostics := runStandaloneAnalysisRule(t, root, inlineErrorDeclarationRule{})
	if len(diagnostics) != 4 {
		t.Fatalf("got %d diagnostics, want 4: %#v", len(diagnostics), diagnostics)
	}
	for _, item := range diagnostics {
		if item.Code != "inline-error-declaration" || item.Severity != diagnostic.SeverityNote {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
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

func TestDeclarationOrderReportsOneRegressionPerFile(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type item struct{}

const first = 1

var current item

func init() { current = item{} }
func use() item { return current }

const late = 2
type later struct{}
`,
	)
	diagnostics := runStandaloneAnalysisRule(t, root, declarationOrderRule{})
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
	if diagnostics[0].Code != "declaration-order" || diagnostics[0].Severity != diagnostic.SeverityNote {
		t.Fatalf("unexpected diagnostic: %#v", diagnostics[0])
	}
}

func TestDeclarationOrderAcceptsTypeConstVarAndFunctions(t *testing.T) {
	root := analysisModule(t, `package sample

type item struct{}

const first = 1

var current item

func init() { current = item{} }
func use() item { return current }
`)
	diagnostics := runStandaloneAnalysisRule(t, root, declarationOrderRule{})
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
}

func TestAdditionalResearchRuleSeverities(t *testing.T) {
	tests := []struct {
		rule Rule
		severity diagnostic.Severity
	}{
		{discardedErrorResultRule{}, diagnostic.SeverityError},
		{inlineErrorDeclarationRule{}, diagnostic.SeverityNote},
		{testParallelismRule{}, diagnostic.SeverityNote},
		{declarationOrderRule{}, diagnostic.SeverityNote},
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
	previous, existed := requirementsByCode[meta.Code]
	requirementsByCode[meta.Code] = Requirements{Stage: AnalysisStageTypes}
	defer func() {
		if existed {
			requirementsByCode[meta.Code] = previous
		} else {
			delete(requirementsByCode, meta.Code)
		}
	}()
	registry := &Registry{rules: []Rule{rule}, settings: map[string]configuredRule{meta.Code: {severity: meta.DefaultSeverity}}, knownCodes: map[string]bool{meta.Code: true}}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	return diagnostics
}
