package lint

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
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
		"cyclomatic-complexity", "max-parameters", "no-defer-in-loop", "no-else-after-return",
		"no-init", "no-naked-return", "no-package-var",
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
	registry, _ := NewRegistry(nil)
	b.ReportAllocs()
	for range b.N {
		if _, err := Run([]string{filename}, registry); err != nil {
			b.Fatal(err)
		}
	}
}
