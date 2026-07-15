package lint

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

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
	for _, wanted := range []string{
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

func TestCatalogIsCompleteDocumentedAndRunnable(t *testing.T) {
	const expectedCount = 111
	const expectedNamesSHA256 = "f8662a9d714c585704b3790d37397fcc2f3e4307cb5f1052689d1aba9ed99d2d"
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
		"rules",
	)
	fixture := writeFixture(t, "// Package p is a fixture.\npackage p\n")
	for _, rule := range rules {
		meta := rule.Meta()
		if seen[meta.Code] {
			t.Errorf("duplicate rule %q", meta.Code)
		}
		seen[meta.Code] = true
		names = append(names, meta.Code)
		if _, err := os.Stat(filepath.Join(docsDirectory, meta.Code+".md")); err != nil {
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
	for _, code := range []string{
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
