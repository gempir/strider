package syntax

import (
	"strings"
	"testing"
)

func TestBidirectionalControlCharacterReportsInvisibleSourceControl(t *testing.T) {
	fixture := writeFixture(t, "package sample\n\n// visible text \u202e hidden ordering\nfunc safe() {}\n")
	registry, err := NewRegistry([]string{
		"bidirectional-control-character",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		fixture,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnosticGolden(t, diagnostics)
}

func TestConcreteNamingChecks(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample

import "fmt"

var _hidden = 1
type record struct { Name int; name int }
func use(fmt int, len int, bad_name int) { _, _, _ = fmt, len, bad_name }
`,
	)
	codes := []string{
		"confusing-naming",
		"import-shadowing",
		"redefines-builtin-id",
		"unexported-naming",
		"var-naming",
	}
	registry, err := NewRegistry(codes)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		fixture,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, item := range diagnostics {
		seen[item.Code] = true
	}
	for _, code := range codes {
		if !seen[code] {
			t.Errorf("%s did not report: %#v", code, diagnostics)
		}
	}
}

func TestConcreteFunctionChecks(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample

type item struct{}

func overloaded(a, b, c, d, e, f, g, h, i int) {}

func Get(
	flag bool,
	ctx int,
	context context.Context,
	wg sync.WaitGroup,
	delaySeconds time.Duration,
	unused int,
) (int, int, error, int) {
	if flag { return 0, 0, nil, 0 }
	return 0, 0, nil, 0
}

func GetNothing() {}
func done() { return }
func (i item) MarshalJSON() ([]byte, error) { _ = i; return nil, nil }
func (other *item) UnmarshalJSON([]byte) error { _ = other; return nil }
`,
	)
	codes := []string{
		"confusing-results",
		"context-as-argument",
		"error-last-result",
		"flag-parameter",
		"function-result-limit",
		"get-function-return-value",
		"marshal-receiver",
		"receiver-naming",
		"redundant-final-return",
		"time-naming",
		"unused-parameter",
		"waitgroup-by-value",
	}
	registry, err := NewRegistry(codes)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		fixture,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, item := range diagnostics {
		seen[item.Code] = true
	}
	for _, code := range codes {
		if !seen[code] {
			t.Errorf("%s did not report: %#v", code, diagnostics)
		}
	}
}

func TestFlagParameterOnlyScansConditionalExpressions(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample

func isReady() bool { return true }
func bodyOnly(flag bool) {
	if isReady() {
		println(flag)
	}
}

func controlsFlow(flag bool) {
	if flag {
		println("ready")
	}
}
`,
	)
	registry, err := NewRegistry([]string{
		"flag-parameter",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		fixture,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnosticGolden(t, diagnostics)
	if !strings.Contains(diagnostics[0].Message, "flag") {
		t.Fatalf("got %#v, want only controlsFlow's flag parameter", diagnostics)
	}
}

func TestUncheckedTypeAssertionRequiresSoleMultiValueRHS(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample

func normalize(string) (string, bool) { return "", false }

func assertions(input any) {
	checked, ok := input.(string)
	checked, ok = input.(string)
	_, _ = checked, ok

	mixed, mixedOK := input.(string), true
	mixed, mixedOK = input.(string), true
	_, _ = mixed, mixedOK

	nested, nestedOK := normalize(input.(string))
	_, _ = nested, nestedOK
}
`,
	)
	registry, err := NewRegistry([]string{
		"unchecked-type-assertion",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		fixture,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnosticGolden(t, diagnostics)
}
