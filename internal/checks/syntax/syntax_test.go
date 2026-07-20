package syntax

import (
	"crypto/sha256"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/diagnostic"
)

func intPointer(value int) *int {
	return &value
}

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
	if len(diagnostics) != 1 || diagnostics[0].Code != "no-package-var" {
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
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
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
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
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
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
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
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
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
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
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
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
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
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
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
	if len(diagnostics) != 4 {
		t.Fatalf("got %d diagnostics, want 4: %#v", len(diagnostics), diagnostics)
	}
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
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
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
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
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
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
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
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
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
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "flag") {
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
	if len(diagnostics) != 3 {
		t.Fatalf("got %d diagnostics, want 3 unchecked assertions: %#v", len(diagnostics), diagnostics)
	}
}

func TestConcreteCallChecks(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample

func helper() {
	runtime.GC()
	_ = errors.New(fmt.Sprintf("value %d", 1))
	_ = fmt.Errorf("static message")
	fmt.Printf("static message")
	print("message")
	sort.Slice(nil, nil)
	os.Exit(1)
	_ = errors.New("Bad message.")
}
`,
	)
	registry, err := NewRegistry(
		[]string{
			"call-to-gc",
			"deep-exit",
			"error-strings",
			"prefer-fmt-errorf",
			"unnecessary-format",
			"use-errors-new",
			"use-fmt-print",
			"use-slices-sort",
		},
	)
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
	wanted := map[string]int{
		"call-to-gc":         1,
		"deep-exit":          1,
		"error-strings":      1,
		"prefer-fmt-errorf":  1,
		"unnecessary-format": 1,
		"use-errors-new":     1,
		"use-fmt-print":      1,
		"use-slices-sort":    1,
	}
	for code, count := range wanted {
		if counts[code] != count {
			t.Errorf("%s produced %d findings; want %d: %#v", code, counts[code], count, diagnostics)
		}
	}
}

func TestConcreteImportChecks(t *testing.T) {
	fixture := writeFixture(t, `package sample

import (
	. "fmt"
	_ "net/http"
	Bad_Alias "strings"
	strings "strings"
)
`)
	registry, err := NewRegistry([]string{
		"blank-imports",
		"dot-imports",
		"duplicated-imports",
		"import-alias-naming",
		"redundant-import-alias",
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
		"blank-imports",
		"dot-imports",
		"duplicated-imports",
		"import-alias-naming",
		"redundant-import-alias",
	} {
		if counts[code] != 1 {
			t.Errorf("%s produced %d findings; want 1: %#v", code, counts[code], diagnostics)
		}
	}
}

func TestExternalTestPackageMatchesNamingAndDirectory(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "sample")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(directory, "sample_test.go")
	if err := os.WriteFile(filename, []byte("package sample_test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry([]string{
		"package-naming",
		"package-directory-mismatch",
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
	if len(diagnostics) != 0 {
		t.Fatalf("standard external test package produced diagnostics: %#v", diagnostics)
	}
}

func TestFileLengthLimitDefaultsTo500AndExplicitZeroDisables(t *testing.T) {
	fixture := writeFixture(t, "package sample\n"+strings.Repeat("// line\n", 500))
	defaultRegistry, err := NewRegistry([]string{
		"file-length-limit",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := defaultRegistry.limits()["file-length-limit"]; got != 500 {
		t.Fatalf("default max lines = %d, want 500", got)
	}
	diagnostics, err := Run([]string{
		fixture,
	}, defaultRegistry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("default file length produced %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
	configuredRegistry, err := NewRegistryWithOptions(
		RegistryOptions{
			Only: []string{
				"file-length-limit",
			},
			Settings: map[string]config.CheckConfig{
				"file-length-limit": {
					MaxLines: intPointer(12),
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := configuredRegistry.limits()["file-length-limit"]; got != 12 {
		t.Fatalf("configured max lines = %d, want 12", got)
	}

	configurationPath := filepath.Join(t.TempDir(), config.Filename)
	contents := "version = 1\n[checks.file-length-limit]\nmax-lines = 0\n"
	if err := os.WriteFile(configurationPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration, err := config.Load(configurationPath, false)
	if err != nil {
		t.Fatal(err)
	}
	setting := configuration.Checks.Settings["file-length-limit"]
	disabledRegistry, err := NewRegistryConfigured([]string{
		"file-length-limit",
	}, map[string]config.CheckConfig{
		"file-length-limit": setting,
	}, configuration.Root)
	if err != nil {
		t.Fatal(err)
	}
	if got := disabledRegistry.limits()["file-length-limit"]; got != 0 {
		t.Fatalf("explicit max lines = %d, want disabled value 0", got)
	}
	diagnostics, err = Run([]string{
		fixture,
	}, disabledRegistry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("explicit max-lines = 0 produced diagnostics: %#v", diagnostics)
	}
}

func TestCatalogIsCompleteDocumentedAndRunnable(t *testing.T) {
	const expectedCount = 97
	const expectedNamesSHA256 = "b47a5f601a71a8d0e6f97b408203e0d638ace7de23422f3f0edeb465bb29d085"
	all, err := NewRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	checks := all.Checks()
	if len(checks) != expectedCount {
		t.Fatalf("catalog has %d checks; want %d", len(checks), expectedCount)
	}
	names := make([]string, 0, len(checks))
	seen := map[string]bool{}
	_, testFile, _, _ := runtime.Caller(0)
	docsDirectory := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "docs", "src", "content", "docs", "lints")
	fixture := writeFixture(t, "// Package p is a fixture.\npackage p\n")
	coreCodes := map[string]bool{
		"cyclomatic-complexity": true,
		"max-parameters":        true,
		"no-defer-in-loop":      true,
		"no-else-after-return":  true,
		"no-init":               true,
		"no-naked-return":       true,
		"no-package-var":        true,
	}
	for _, check := range checks {
		meta := check.Meta()
		if seen[meta.Code] {
			t.Errorf("duplicate check %q", meta.Code)
		}
		seen[meta.Code] = true
		names = append(names, meta.Code)
		if strings.TrimSpace(meta.GoodExample) == "" || strings.TrimSpace(meta.BadExample) == "" {
			t.Errorf("check %s has incomplete examples", meta.Code)
		}
		if strings.HasPrefix(meta.GoodExample, "See the check reference") || strings.HasPrefix(meta.BadExample, "See the check reference") {
			t.Errorf("check %s still has placeholder examples", meta.Code)
		}
		if !coreCodes[meta.Code] && (meta.Explanation == meta.Summary+"." || !strings.Contains(meta.Explanation, "Default:")) {
			t.Errorf("extended check %s explanation does not add its default contract: %q", meta.Code, meta.Explanation)
		}
		if _, err := os.Stat(filepath.Join(docsDirectory, meta.Code+".md")); err != nil {
			t.Errorf("check %s has no documentation: %v", meta.Code, err)
		}
		registry, err := NewRegistry([]string{
			meta.Code,
		})
		if err != nil {
			t.Errorf("select %s: %v", meta.Code, err)
			continue
		}
		if _, err := Run([]string{
			fixture,
		}, registry); err != nil {
			t.Errorf("run %s: %v", meta.Code, err)
		}
	}
	sort.Strings(names)
	digest := sha256.Sum256([]byte(strings.Join(names, "\n") + "\n"))
	if got := fmt.Sprintf("%x", digest); got != expectedNamesSHA256 {
		t.Errorf("check-name catalog changed: got digest %s", got)
	}
	if got, want := len(all.Checks()), expectedCount; got != want {
		t.Errorf("registry contains %d checks; want %d", got, want)
	}
}

func TestEveryLintCheckAcceptsCommonConfiguration(t *testing.T) {
	all, err := NewRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	settings := make(map[string]config.CheckConfig, len(all.Checks()))
	for _, check := range all.Checks() {
		settings[check.Meta().Code] = config.CheckConfig{
			Severity: "note",
		}
	}
	configured, err := NewRegistryConfigured(nil, settings, "")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(configured.Checks()), len(all.Checks()); got != want {
		t.Fatalf("configured %d checks; want %d", got, want)
	}
	for _, check := range configured.Checks() {
		if severity := configured.Severity(check.Meta().Code); severity != diagnostic.SeverityNote {
			t.Errorf("%s severity = %s", check.Meta().Code, severity)
		}
	}
}

func TestBannedCharactersUsesDefaultsAndConfiguration(t *testing.T) {
	filename := writeFixture(t, "package p\nvar ᐸname, under_score int\n")
	for name, test := range map[string]struct {
		settings map[string]config.CheckConfig
		wanted   string
	}{
		"defaults": {
			wanted: "ᐸ",
		},
		"configured": {
			settings: map[string]config.CheckConfig{
				"banned-characters": {
					Characters: []string{
						"_",
					},
				},
			},
			wanted: "_",
		},
	} {
		t.Run(
			name,
			func(t *testing.T) {
				registry, err := NewRegistryWithOptions(RegistryOptions{
					Only: []string{
						"banned-characters",
					},
					Settings: test.settings,
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
				if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, test.wanted) {
					t.Fatalf("diagnostics = %#v, want one finding for %q", diagnostics, test.wanted)
				}
				if diagnostics[0].Severity != diagnostic.SeverityError {
					t.Fatalf("severity = %s, want error", diagnostics[0].Severity)
				}
			},
		)
	}
}

func TestBannedCharactersRejectsInvalidConfiguration(t *testing.T) {
	for name, settings := range map[string]map[string]config.CheckConfig{
		"multiple runes": {
			"banned-characters": {
				Characters: []string{
					"ab",
				},
			},
		},
		"unrelated check": {
			"no-init": {
				Characters: []string{
					"_",
				},
			},
		},
	} {
		t.Run(
			name,
			func(t *testing.T) {
				if _, err := NewRegistryWithOptions(RegistryOptions{
					Settings: settings,
				}); err == nil {
					t.Fatal("expected invalid character configuration to fail")
				}
			},
		)
	}
}

func TestLintRegistryFiltersByEffectiveSeverityBeforeExecution(t *testing.T) {
	for name, options := range map[string]RegistryOptions{
		"only": {
			Only: []string{
				"no-init",
			},
			Settings: map[string]config.CheckConfig{
				"no-init": {
					Severity: "warning",
				},
			},
			MinimumSeverity: diagnostic.SeverityError,
		},
		"all": {
			Settings: map[string]config.CheckConfig{
				"no-init": {
					Severity: "warning",
				},
			},
			MinimumSeverity: diagnostic.SeverityError,
		},
	} {
		t.Run(
			name,
			func(t *testing.T) {
				registry, err := NewRegistryWithOptions(options)
				if err != nil {
					t.Fatal(err)
				}
				for _, check := range registry.Checks() {
					if check.Meta().Code == "no-init" {
						t.Fatal("selection bypassed the minimum severity")
					}
				}
				if name == "only" {
					diagnostics, runErr := Run([]string{
						filepath.Join(t.TempDir(), "missing.go"),
					}, registry)
					if runErr != nil {
						t.Fatalf("empty registry attempted CST execution: %v", runErr)
					}
					if diagnostics == nil || len(diagnostics) != 0 {
						t.Fatalf("empty registry diagnostics = %#v", diagnostics)
					}
				}
			},
		)
	}

	registry, err := NewRegistryWithOptions(
		RegistryOptions{
			Only: []string{
				"no-init",
			},
			Settings: map[string]config.CheckConfig{
				"no-init": {
					Severity: "error",
				},
			},
			MinimumSeverity: diagnostic.SeverityError,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(registry.Checks()); got != 1 {
		t.Fatalf("checks after severity override = %d, want 1", got)
	}
	if severity := registry.Severity("no-init"); severity != diagnostic.SeverityError {
		t.Fatalf("effective severity = %s, want error", severity)
	}
}

func TestLintRegistryRejectsInvalidMinimumSeverity(t *testing.T) {
	_, err := NewRegistryWithOptions(RegistryOptions{
		MinimumSeverity: "fatal",
	})
	if err == nil || !strings.Contains(err.Error(), "minimum severity") {
		t.Fatalf("got %v, want minimum severity error", err)
	}
	_, err = NewRegistryWithOptions(RegistryOptions{
		Settings: map[string]config.CheckConfig{
			"no-init": {
				Severity: "fatal",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "severity must be") {
		t.Fatalf("got %v, want check severity error", err)
	}
}

func TestLintCheckConfigurationCanExcludePaths(t *testing.T) {
	fixture := writeFixture(t, "package p\nfunc init() {}\n")
	registry, err := NewRegistryConfigured([]string{
		"no-init",
	}, map[string]config.CheckConfig{
		"no-init": {
			Excludes: []string{
				"**/*.go",
			},
		},
	}, filepath.Dir(fixture))
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		fixture,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics == nil {
		t.Fatal("clean single-file lint returned a nil diagnostics slice")
	}
	if len(diagnostics) != 0 {
		t.Fatalf("excluded check reported diagnostics: %#v", diagnostics)
	}
}

func TestRunWithoutFilesReturnsEmptyDiagnostics(t *testing.T) {
	registry, err := NewRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run(nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics == nil || len(diagnostics) != 0 {
		t.Fatalf("got %#v, want a non-nil empty diagnostics slice", diagnostics)
	}
}

func TestExtendedNativeChecksReportRepresentativeFindings(t *testing.T) {
	source := `// Package sample demonstrates compatibility checks.
package sample
import (
	"errors"
	"fmt"
	"runtime"
	"time"
)
var ErrBad = errors.New("Bad message.")
func TooMany(a,b,c,d,e,f,g,h,i bool) (int,int,int,int) {
	if a == true { runtime.GC() } else { runtime.GC() }
	fmt.Errorf("static error")
	_ = time.Date(2020, 13, 1, 0, 0, 0, 0, time.UTC)
	return 1, 2, 3, 4
}
func Named() (result int) { return }
func Convert() string { return string(123) }
func Assert(v interface{}) { _ = v.(string) }
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
	codes := map[string]bool{}
	for _, item := range diagnostics {
		codes[item.Code] = true
	}
	for _, code := range []string{
		"boolean-literal-comparison",
		"call-to-gc",
		"error-strings",
		"function-result-limit",
		"identical-branches",
		"no-naked-return",
		"string-of-int",
		"time-date",
		"unchecked-type-assertion",
		"use-any",
		"use-errors-new",
	} {
		if !codes[code] {
			t.Errorf("expected %s finding; got codes %v", code, codes)
		}
	}
}

func TestCSTPolicyChecksReportRepresentativeFindings(t *testing.T) {
	filename := writeFixture(
		t,
		`package sample
import (
	"sync"
	"sync/atomic"
)
var counter int64
type item struct{ count int }
func (current item) mutate(value int, group sync.WaitGroup, closer interface{ Close() error }) {
	value = 2
	current.count++
	counter = atomic.AddInt64(&counter, 1)
	values := map[string]int{"answer": 42}
	if found, ok := values["answer"]; ok {
		_ = found
		_ = values["answer"]
	}
	_ = group
	_ = closer
}
`,
	)
	registry, err := NewRegistry([]string{
		"redundant-atomic-result-assignment",
		"inefficient-map-lookup",
		"modifies-parameter",
		"modifies-value-receiver",
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
	codes := map[string]bool{}
	for _, item := range diagnostics {
		codes[item.Code] = true
	}
	for _, code := range []string{
		"redundant-atomic-result-assignment",
		"inefficient-map-lookup",
		"modifies-parameter",
		"modifies-value-receiver",
	} {
		if !codes[code] {
			t.Errorf("expected %s finding; got codes %v", code, codes)
		}
	}
}

func TestExtendedCheckOrderingIsDeterministic(t *testing.T) {
	filename := writeFixture(t, `package p
func f(values map[string]int) {
	for key, value := range values {
		_, _ = &key, &value
	}
}
`)
	registry, err := NewRegistry([]string{
		"range-value-address",
	})
	if err != nil {
		t.Fatal(err)
	}
	var expected []diagnostic.Diagnostic
	for range 20 {
		diagnostics, err := Run([]string{
			filename,
		}, registry)
		if err != nil {
			t.Fatal(err)
		}
		if expected == nil {
			expected = diagnostics
			continue
		}
		if !reflect.DeepEqual(diagnostics, expected) {
			t.Fatalf("diagnostic order changed:\nfirst: %#v\nnext: %#v", expected, diagnostics)
		}
	}
}

func writeFixture(t *testing.T, source string) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "fixture.go")
	if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return filename
}

func BenchmarkLint(b *testing.B) {
	filename := filepath.Join(b.TempDir(), "fixture.go")
	if err := os.WriteFile(filename, []byte("package p\nfunc F(a int) int { if a > 0 { return a }; return -a }\n"), 0o600); err != nil {
		b.Fatal(err)
	}
	registry, err := NewRegistry(nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for range b.N {
		if _, err := Run([]string{
			filename,
		}, registry); err != nil {
			b.Fatal(err)
		}
	}
}

func TestAnalyzeTreeMatchesFileRun(t *testing.T) {
	filename := writeFixture(t, "package p\nfunc init() {}\n")
	registry, err := NewRegistry([]string{
		"no-init",
	})
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := cst.Parse(filename, contents)
	if err != nil {
		t.Fatal(err)
	}
	fromTree := AnalyzeTree(filename, tree, registry)
	fromFile, err := Run([]string{
		filename,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fromTree, fromFile) {
		t.Fatalf("tree diagnostics %#v differ from file diagnostics %#v", fromTree, fromFile)
	}
}

func TestSortDiagnosticsUsesEndOffsetAsTieBreaker(t *testing.T) {
	diagnostics := []diagnostic.Diagnostic{
		{
			File:    "main.go",
			Code:    "example",
			Message: "same",
			Start: token.Position{
				Offset: 10,
			},
			End: token.Position{
				Offset: 30,
			},
		},
		{
			File:    "main.go",
			Code:    "example",
			Message: "same",
			Start: token.Position{
				Offset: 10,
			},
			End: token.Position{
				Offset: 20,
			},
		},
	}
	sortDiagnostics(diagnostics)
	if diagnostics[0].End.Offset != 20 || diagnostics[1].End.Offset != 30 {
		t.Fatalf("diagnostics not totally ordered by range: %#v", diagnostics)
	}
}
