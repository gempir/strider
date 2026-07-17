package lint

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
	"github.com/gempir/strider/internal/diagnostic"
)

func TestInitialRules(t *testing.T) {
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
	diagnostics, err := Run([]string{filename}, registry)
	if err != nil {
		t.Fatal(err)
	}
	codes := make([]string, 0, len(diagnostics))
	for _, item := range diagnostics {
		codes = append(codes, item.Code)
	}
	for _, wanted := range[]string{
		"cyclomatic-complexity",
		"max-parameters",
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

func TestDefaultProfileUsesConcreteSyntaxAndExactRanges(t *testing.T) {
	source := `package p

func overloaded[T any](one, two T, three, four, five, six int) {}

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
	registry, err := NewRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{filename}, registry)
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
		t.Errorf(
			"nested function boundary produced %d defer findings; want 1",
			counts["no-defer-in-loop"],
		)
	}
	if counts["no-naked-return"] != 1 {
		t.Errorf(
			"named generic result produced %d naked-return findings; want 1",
			counts["no-naked-return"],
		)
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
	registry, _ := NewRegistry(nil)
	diagnostics, err := Run([]string{filename}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "no-package-var" {
		t.Fatalf("got diagnostics %#v", diagnostics)
	}
}

func TestOnlyAndUnknownRule(t *testing.T) {
	registry, err := NewRegistry([]string{"no-init"})
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Rules()) != 1 || registry.Rules()[0].Meta().Code != "no-init" {
		t.Fatalf("unexpected registry: %#v", registry.Rules())
	}
	if _, err := NewRegistry([]string{"not-a-rule"}); err == nil {
		t.Fatal("expected unknown rule error")
	}
}

func TestIneffectivePointerCopyReportsPointerRoundTrips(t *testing.T) {
	fixture := writeFixture(
		t,
		"package sample\nfunc copy(pointer *int, value int) { _ = &*pointer; _ = *&value }\n",
	)
	registry, err := NewRegistry([]string{"ineffective-pointer-copy"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestDoubleNegationReportsRedundantBooleanNegation(t *testing.T) {
	fixture := writeFixture(t, "package sample\nfunc ready(value bool) bool { return !!value }\n")
	registry, err := NewRegistry([]string{"double-negation"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
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
	registry, err := NewRegistry([]string{"identical-ifelseif-conditions"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
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
	registry, err := NewRegistry([]string{"redundant-build-tag"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestZeroIntegerDivisionReportsLiteralTruncation(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample

func ratio() int { return 2 / 3 }
func useful() int { return 4 / 3 }
func floating() float64 { return 2 / 3.0 }
`,
	)
	registry, err := NewRegistry([]string{"zero-integer-division"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestModuloOneReportsConstantZeroRemainder(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample

func remainder(value int) int { return value % 1 }
func useful(value int) int { return value % 2 }
`,
	)
	registry, err := NewRegistry([]string{"modulo-one"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestSpinningSelectDefaultReportsEmptyDefault(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample

func spin(messages <-chan string) {
	for {
		select {
		case <-messages:
		default:
		}
	}
}
`,
	)
	registry, err := NewRegistry([]string{"spinning-select-default"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
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
	A string ` + "`json:\"a\" json:\"b\"`" + `
	B string ` + "`xml:\"b,attr,attr\"`" + `
	C string ` + "`xml:\"c,mystery\"`" + `
	D string ` + "`json:\"d,omitEmpty\"`" + `
}
`,
	)
	registry, err := NewRegistry([]string{"struct-tag"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
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
	registry, err := NewRegistry([]string{"range"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
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
	registry, err := NewRegistry([]string{"empty-block"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestSpacedCompilerDirectiveReportsIgnoredDirective(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample

// go:noinline
func ignored() {}

//go:noinline
func active() {}

func local() { // go:noinline
}
`,
	)
	registry, err := NewRegistry([]string{"spaced-compiler-directive"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestDeferAllowsReturnedFunctionInvocation(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample

func setup() func() { return func() {} }
func run() { defer setup()() }
`,
	)
	registry, err := NewRegistry([]string{"defer"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("got unexpected diagnostics: %#v", diagnostics)
	}
}

func TestBidirectionalControlCharacterReportsInvisibleSourceControl(t *testing.T) {
	fixture := writeFixture(
		t,
		"package sample\n\n// visible text \u202e hidden ordering\nfunc safe() {}\n",
	)
	registry, err := NewRegistry([]string{"bidirectional-control-character"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestConcreteNamingRules(t *testing.T) {
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
	diagnostics, err := Run([]string{fixture}, registry)
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

func TestConcreteFunctionRules(t *testing.T) {
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
		"argument-limit",
		"confusing-results",
		"context-as-argument",
		"error-return",
		"flag-parameter",
		"function-result-limit",
		"get-return",
		"marshal-receiver",
		"receiver-naming",
		"time-naming",
		"unnecessary-stmt",
		"unused-parameter",
		"waitgroup-by-value",
	}
	registry, err := NewRegistry(codes)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
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

func TestConcreteCallRules(t *testing.T) {
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
			"errorf",
			"unnecessary-format",
			"use-errors-new",
			"use-fmt-print",
			"use-slices-sort",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, item := range diagnostics {
		counts[item.Code]++
	}
	wanted := map[string]int{
		"call-to-gc": 1,
		"deep-exit": 1,
		"error-strings": 2,
		"errorf": 1,
		"unnecessary-format": 1,
		"use-errors-new": 1,
		"use-fmt-print": 1,
		"use-slices-sort": 1,
	}
	for code, count := range wanted {
		if counts[code] != count {
			t.Errorf(
				"%s produced %d findings; want %d: %#v",
				code,
				counts[code],
				count,
				diagnostics,
			)
		}
	}
}

func TestConcreteImportRules(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample

import (
	. "fmt"
	_ "net/http"
	Bad_Alias "strings"
	strings "strings"
)
`,
	)
	registry, err := NewRegistry(
		[]string{
			"blank-imports",
			"dot-imports",
			"duplicated-imports",
			"import-alias-naming",
			"redundant-import-alias",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, item := range diagnostics {
		counts[item.Code]++
	}
	for _, code := range[]string{
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

func TestCatalogIsCompleteDocumentedAndRunnable(t *testing.T) {
	const expectedCount = 116
	const expectedNamesSHA256 = "915035c9aeff444086db5beafb9f0dcae18ba97e187850c26fee428666e2ae75"
	all, err := NewRegistryAll()
	if err != nil {
		t.Fatal(err)
	}
	rules := all.Rules()
	if len(rules) != expectedCount {
		t.Fatalf("catalog has %d rules; want %d", len(rules), expectedCount)
	}
	names := make([]string, 0, len(rules))
	seen := map[string]bool{}
	_, testFile, _, _ := runtime.Caller(0)
	docsDirectory := filepath.Join(
		filepath.Dir(testFile),
		"..",
		"..",
		"docs",
		"src",
		"content",
		"docs",
		"lints",
	)
	fixture := writeFixture(t, "// Package p is a fixture.\npackage p\n")
	for _, rule := range rules {
		meta := rule.Meta()
		if seen[meta.Code] {
			t.Errorf("duplicate rule %q", meta.Code)
		}
		seen[meta.Code] = true
		names = append(names, meta.Code)
		if strings.TrimSpace(meta.GoodExample) == "" || strings.TrimSpace(meta.BadExample) == "" {
			t.Errorf("rule %s has incomplete examples", meta.Code)
		}
		if strings.HasPrefix(meta.GoodExample, "See the rule reference") || strings.HasPrefix(
			meta.BadExample,
			"See the rule reference",
		) {
			t.Errorf("rule %s still has placeholder examples", meta.Code)
		}
		if _, err := os.Stat(filepath.Join(docsDirectory, meta.Code + ".md")); err != nil {
			t.Errorf("rule %s has no documentation: %v", meta.Code, err)
		}
		registry, err := NewRegistry([]string{meta.Code})
		if err != nil {
			t.Errorf("select %s: %v", meta.Code, err)
			continue
		}
		if _, err := Run([]string{fixture}, registry); err != nil {
			t.Errorf("run %s: %v", meta.Code, err)
		}
	}
	sort.Strings(names)
	digest := sha256.Sum256([]byte(strings.Join(names, "\n") + "\n"))
	if got := fmt.Sprintf("%x", digest); got != expectedNamesSHA256 {
		t.Errorf("rule-name catalog changed: got digest %s", got)
	}
	if got, want := len(all.Rules()), expectedCount; got != want {
		t.Errorf("all-rules registry contains %d rules; want %d", got, want)
	}
}

func TestEveryLintRuleAcceptsCommonConfiguration(t *testing.T) {
	all, err := NewRegistryAll()
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	settings := make(map[string]config.RuleConfig, len(all.Rules()))
	for _, rule := range all.Rules() {
		settings[rule.Meta().Code] = config.RuleConfig{Enabled: &enabled, Severity: "note"}
	}
	configured, err := NewRegistryConfigured(nil, false, settings, "")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(configured.Rules()), len(all.Rules()); got != want {
		t.Fatalf("configured %d rules; want %d", got, want)
	}
	for _, rule := range configured.Rules() {
		if severity := configured.Severity(rule.Meta().Code); severity != diagnostic.SeverityNote {
			t.Errorf("%s severity = %s", rule.Meta().Code, severity)
		}
	}
}

func TestLintRuleConfigurationCanExcludePaths(t *testing.T) {
	fixture := writeFixture(t, "package p\nfunc init() {}\n")
	enabled := true
	registry, err := NewRegistryConfigured(
		nil,
		false,
		map[string]config.RuleConfig{"no-init": {Enabled: &enabled, Excludes: []string{"**/*.go"}}},
		filepath.Dir(fixture),
	)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{fixture}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("excluded rule reported diagnostics: %#v", diagnostics)
	}
}

func TestExtendedNativeRulesReportRepresentativeFindings(t *testing.T) {
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
	registry, err := NewRegistryAll()
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{filename}, registry)
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, item := range diagnostics {
		codes[item.Code] = true
	}
	for _, code := range[]string{
		"argument-limit",
		"bare-return",
		"bool-literal-in-expr",
		"call-to-gc",
		"error-strings",
		"function-result-limit",
		"identical-branches",
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

func TestCSTPolicyRulesReportRepresentativeFindings(t *testing.T) {
	filename := writeFixture(
		t,
		`package sample
import (
	"sync"
	"sync/atomic"
	"time"
)
var counter int64
type item struct{ count int }
func (current item) mutate(value int, group sync.WaitGroup, closer interface{ Close() error }) {
	value = 2
	current.count++
	counter = atomic.AddInt64(&counter, 1)
	epoch := time.Now().Unix()
	_ = epoch
	values := map[string]int{"answer": 42}
	if found, ok := values["answer"]; ok {
		_ = found
		_ = values["answer"]
	}
	group.Go(func() { group.Done() })
	_ = time.Now() == time.Now()
	closer.Close()
}
`,
	)
	registry, err := NewRegistry(
		[]string{
			"atomic",
			"epoch-naming",
			"forbidden-call-in-wg-go",
			"inefficient-map-lookup",
			"modifies-parameter",
			"modifies-value-receiver",
			"time-equal",
			"unhandled-error",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{filename}, registry)
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, item := range diagnostics {
		codes[item.Code] = true
	}
	for _, code := range[]string{
		"atomic",
		"epoch-naming",
		"forbidden-call-in-wg-go",
		"inefficient-map-lookup",
		"modifies-parameter",
		"modifies-value-receiver",
		"time-equal",
		"unhandled-error",
	} {
		if !codes[code] {
			t.Errorf("expected %s finding; got codes %v", code, codes)
		}
	}
}

func TestExtendedRuleOrderingIsDeterministic(t *testing.T) {
	filename := writeFixture(
		t,
		`package p
func f(values map[string]int) {
	for key, value := range values {
		_ = func() any { return []any{key, value} }
	}
}
`,
	)
	registry, err := NewRegistry([]string{"datarace", "range-val-in-closure"})
	if err != nil {
		t.Fatal(err)
	}
	var expected []diagnostic.Diagnostic
	for range 20 {
		diagnostics, err := Run([]string{filename}, registry)
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
	if err := os.WriteFile(
		filename,
		[]byte("package p\nfunc F(a int) int { if a > 0 { return a }; return -a }\n"),
		0o600,
	); err != nil {
		b.Fatal(err)
	}
	registry, _ := NewRegistry(nil)
	b.ReportAllocs()
	for range b.N {
		if _, err := Run([]string{filename}, registry); err != nil {
			b.Fatal(err)
		}
	}
}

func TestSortDiagnosticsUsesEndOffsetAsTieBreaker(t *testing.T) {
	diagnostics := []diagnostic.Diagnostic{
		{
			File: "main.go",
			Code: "example",
			Message: "same",
			Start: token.Position{Offset: 10},
			End: token.Position{Offset: 30},
		},
		{
			File: "main.go",
			Code: "example",
			Message: "same",
			Start: token.Position{Offset: 10},
			End: token.Position{Offset: 20},
		},
	}
	sortDiagnostics(diagnostics)
	if diagnostics[0].End.Offset != 20 || diagnostics[1].End.Offset != 30 {
		t.Fatalf("diagnostics not totally ordered by range: %#v", diagnostics)
	}
}
