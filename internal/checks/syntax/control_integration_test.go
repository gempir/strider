package syntax

import (
	"slices"
	"strings"
	"testing"
)

func TestInitialChecks(t *testing.T) {
	source := `package p
var global = 1
func init() {}
func many(a, b, c, d, e, f int) {}
func named() (result int) { result = 1; return }
func loop() { for i := 0; i < 1; i++ { defer closeThing() } }
func nesting(v int) int {
	if v == 0 { return 0 } else { return 1 }
}
func complex(a, b bool, n int) int {
	if a && b { n++ }
	if a || b { n++ }
	for n < 10 { n++ }
	for range []int{1} { n++ }
	switch n { case 1: n++; case 2: n++; case 3: n++ }
	if n > 20 { n-- }
	return n
}
func closeThing() {}
`
	filename := writeFixture(t, source)
	registry, err := NewRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		filename,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	codes := make([]string, 0, len(diagnostics))
	for _, item := range diagnostics {
		codes = append(codes, item.Code)
	}
	for _, wanted := range []string{
		"cyclomatic-complexity",
		"no-defer-in-loop",
		"no-else-after-return",
		"no-init",
		"no-naked-return",
		"no-package-var",
	} {
		if !slices.Contains(codes, wanted) {
			t.Errorf("missing %s in %v", wanted, codes)
		}
	}
}

func TestSyntaxChecksUseConcreteSyntaxAndExactRanges(t *testing.T) {
	source := `package p

func overloaded[T any](one, two T, three, four, five, six, seven, eight, nine int) {}

func named[T any]() (result T) {
	for {
		func() { defer closeThing() }()
		defer closeThing()
		return
	}
}

func closeThing() {}
`
	filename := writeFixture(t, source)
	registry, err := NewRegistry([]string{
		"max-parameters",
		"no-defer-in-loop",
		"no-naked-return",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		filename,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, item := range diagnostics {
		counts[item.Code]++
		selected := source[item.Start.Offset:item.End.Offset]
		if strings.TrimSpace(selected) == "" {
			t.Errorf("%s selected an empty range: %#v", item.Code, item)
		}
	}
	if counts["max-parameters"] != 1 {
		t.Errorf("grouped parameter count produced %d findings; want 1", counts["max-parameters"])
	}
	if counts["no-defer-in-loop"] != 1 {
		t.Errorf("nested function boundary produced %d defer findings; want 1", counts["no-defer-in-loop"])
	}
	if counts["no-naked-return"] != 1 {
		t.Errorf("named generic result produced %d naked-return findings; want 1", counts["no-naked-return"])
	}
}

func TestSuppressions(t *testing.T) {
	source := `//strider:ignore-file no-init
package p
func init() {}
//strider:ignore no-package-var
var allowed = 1
var reported = 2
func loop() {
	for {
		func() { defer closeThing() }()
		break
	}
}
func closeThing() {}
`
	filename := writeFixture(t, source)
	registry, err := NewRegistry([]string{
		"no-init",
		"no-package-var",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		filename,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnosticGolden(t, diagnostics)
	if diagnostics[0].Code != "no-package-var" {
		t.Fatalf("got diagnostics %#v", diagnostics)
	}
}

func TestOnlyAndUnknownCheck(t *testing.T) {
	registry, err := NewRegistry([]string{
		"no-init",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Checks()) != 1 || registry.Checks()[0].Meta().Code != "no-init" {
		t.Fatalf("unexpected registry: %#v", registry.Checks())
	}
	if _, err := NewRegistry([]string{
		"not-a-check",
	}); err == nil {
		t.Fatal("expected unknown check error")
	}
}

func TestIneffectivePointerCopyReportsPointerRoundTrips(t *testing.T) {
	fixture := writeFixture(t, "package sample\nfunc copy(pointer *int, value int) { _ = &*pointer; _ = *&value }\n")
	registry, err := NewRegistry([]string{
		"ineffective-pointer-copy",
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

func TestDoubleNegationReportsRedundantBooleanNegation(t *testing.T) {
	fixture := writeFixture(t, "package sample\nfunc ready(value bool) bool { return !!value }\n")
	registry, err := NewRegistry([]string{
		"double-negation",
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

func TestIdenticalIfElseIfConditionsRequireSideEffectFreeChain(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample

func pure(value int) int {
	if value > 0 {
		return 1
	} else if value > 0 {
		return 2
	}
	return 0
}

func changing(next func() bool) int {
	if next() {
		return 1
	} else if next() {
		return 2
	}
	return 0
}
`,
	)
	registry, err := NewRegistry([]string{
		"identical-if-chain-conditions",
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

func TestRedundantBuildTagReportsEquivalentLegacyConstraints(t *testing.T) {
	fixture := writeFixture(t, `// +build linux darwin
// +build darwin linux

package sample
`)
	registry, err := NewRegistry([]string{
		"redundant-build-tag",
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

func TestZeroIntegerDivisionReportsLiteralTruncation(t *testing.T) {
	fixture := writeFixture(t, `package sample

func ratio() int { return 2 / 3 }
func useful() int { return 4 / 3 }
func floating() float64 { return 2 / 3.0 }
`)
	registry, err := NewRegistry([]string{
		"zero-integer-division",
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

func TestModuloOneReportsConstantZeroRemainder(t *testing.T) {
	fixture := writeFixture(t, `package sample

func remainder(value int) int { return value % 1 }
func useful(value int) int { return value % 2 }
`)
	registry, err := NewRegistry([]string{
		"modulo-one",
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

func TestSpinningSelectDefaultReportsEmptyDefault(t *testing.T) {
	fixture := writeFixture(t, `package sample

func spin(messages <-chan string) {
	for {
		select {
		case <-messages:
		default:
		}
	}
}
`)
	registry, err := NewRegistry([]string{
		"spinning-select-default",
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

func TestStructTagReportsDuplicateKeysAndInvalidOptions(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample
type tagged struct {
	A string `+"`json:\"a\" json:\"b\"`"+`
	B string `+"`xml:\"b,attr,attr\"`"+`
	C string `+"`xml:\"c,mystery\"`"+`
	D string `+"`json:\"d,omitEmpty\"`"+`
}
`,
	)
	registry, err := NewRegistry([]string{
		"invalid-struct-tag",
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

func TestRangeReportsUnnecessaryRuneSliceConversion(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample

func runes(text string) {
	for _, value := range []rune(text) {
		_ = value
	}
	for index, value := range []rune(text) {
		_, _ = index, value
	}
}
`,
	)
	registry, err := NewRegistry([]string{
		"simplify-range",
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

func TestEmptyBlockReportsConditionalBranchesOnly(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample

type marker struct{}
func (marker) mark() {}

func branches(value bool) {
	if value {
	}
	if value {
		println(value)
	} else {
	}
}
`,
	)
	registry, err := NewRegistry([]string{
		"empty-conditional-block",
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

func TestSpacedCompilerDirectiveReportsIgnoredDirective(t *testing.T) {
	fixture := writeFixture(t, `package sample

// go:noinline
func ignored() {}

//go:noinline
func active() {}

func local() { // go:noinline
}
`)
	registry, err := NewRegistry([]string{
		"spaced-compiler-directive",
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

func TestDeferAllowsReturnedFunctionInvocation(t *testing.T) {
	fixture := writeFixture(t, `package sample

func setup() func() { return func() {} }
func run() { defer setup()() }
`)
	registry, err := NewRegistry([]string{
		"deferred-recover-call",
		"discarded-deferred-result",
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
	if len(diagnostics) != 0 {
		t.Fatalf("got unexpected diagnostics: %#v", diagnostics)
	}
}

func TestDeferMistakesUseSpecificCodes(t *testing.T) {
	fixture := writeFixture(t, `package sample

func cleanup() error { return nil }
func run() {
	defer recover()
	defer func() error { return cleanup() }()
}
`)
	registry, err := NewRegistry([]string{
		"deferred-recover-call",
		"discarded-deferred-result",
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
	counts := map[string]int{}
	for _, item := range diagnostics {
		counts[item.Code]++
	}
	for _, code := range []string{
		"deferred-recover-call",
		"discarded-deferred-result",
	} {
		if counts[code] != 1 {
			t.Errorf("%s produced %d findings; want 1: %#v", code, counts[code], diagnostics)
		}
	}
}

func TestRedundantStatementsUseSpecificCodes(t *testing.T) {
	fixture := writeFixture(t, `package sample

func final() { return }
func choose(value int) {
	switch value {
	case 1:
		break
	}
}
`)
	codes := []string{
		"redundant-final-return",
		"redundant-switch-break",
		"single-case-switch",
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
	counts := map[string]int{}
	for _, item := range diagnostics {
		counts[item.Code]++
	}
	for _, code := range codes {
		if counts[code] != 1 {
			t.Errorf("%s produced %d findings; want 1: %#v", code, counts[code], diagnostics)
		}
	}
}
