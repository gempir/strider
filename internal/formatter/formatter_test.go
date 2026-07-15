package formatter

import (
	"errors"
	"strings"
	"testing"
)

func TestFormatTypicalApplicationCode(t *testing.T) {
	input := `// Package p does things.
package p
import (
 "strings"
 "fmt"
)
// VeryLongFunctionName formats a value.
func VeryLongFunctionName(firstParameter string, secondParameter string, thirdParameter string, fourthParameter string) string {
// retain this comment
value:=strings.TrimSpace(firstParameter)
if value=="" { return secondParameter } else { return fmt.Sprint(thirdParameter, fourthParameter) }
}
`
	want := `// Package p does things.
package p

import (
	"fmt"
	"strings"
)

// VeryLongFunctionName formats a value.
func VeryLongFunctionName(
	firstParameter string,
	secondParameter string,
	thirdParameter string,
	fourthParameter string,
) string {
	// retain this comment
	value := strings.TrimSpace(firstParameter)
	if value == "" {
		return secondParameter
	} else {
		return fmt.Sprint(thirdParameter, fourthParameter)
	}
}
`

	result, err := Format("example.go", []byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatal("expected input to change")
	}
	if got := string(result.Source); got != want {
		t.Fatalf("formatted source:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatRefusesUnsupportedSyntax(t *testing.T) {
	tests := map[string]string{
		"generics": "package p\nfunc F[T any](v T) T { return v }\n",
		"goto":     "package p\nfunc F() { goto done; done: return }\n",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Format(name+".go", []byte(input))
			if !errors.Is(err, ErrUnsupported) {
				t.Fatalf("got %v, want ErrUnsupported", err)
			}
		})
	}
}

func TestFormatIgnoreReturnsExactInput(t *testing.T) {
	input := []byte("package p\n//strider:format-ignore\nfunc F( ){ }\n")
	result, err := Format("ignored.go", input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ignored || result.Changed || string(result.Source) != string(input) {
		t.Fatalf("unexpected ignored result: %#v", result)
	}
}

func TestFormatPreservesBuildConstraintSeparation(t *testing.T) {
	input := []byte("//go:build linux\n\n// Package p is a fixture.\npackage p\nfunc F(){return}\n")
	result, err := Format("constraint.go", input)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := "//go:build linux\n\n// Package p is a fixture.\npackage p\n"
	if !strings.HasPrefix(string(result.Source), wantPrefix) {
		t.Fatalf("build constraint moved:\n%s", result.Source)
	}
}

func TestFormatPreservesSwitchComments(t *testing.T) {
	input := []byte(`package p
func F(v int) int {
	switch v {
	// the first case
	case 1:
		return 1
	default: // fallback
		return 0
	}
}
`)
	result, err := Format("switch.go", input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.Source), "// the first case\n\tcase 1:") ||
		!strings.Contains(string(result.Source), "default: // fallback") {
		t.Fatalf("case comments moved:\n%s", result.Source)
	}
}

func TestFormatTypeSwitchAndChannelSend(t *testing.T) {
	input := []byte(`package p

func F(value any, output chan string) {
	switch current := value.(type) {
	case string:
		output <- current
	default:
		output <- "unknown"
	}
}
`)
	result, err := Format("concurrency.go", input)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(result.Source); got != string(input) {
		t.Fatalf("formatted source:\n%s\nwant:\n%s", got, input)
	}
}

func TestFormatSelect(t *testing.T) {
	input := []byte(`package p

func F(input <-chan string, output chan<- string) {
	select {
	case value := <-input: // received
		output <- value
	case output <- "fallback":
	default:
	}
}
`)
	result, err := Format("select.go", input)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(result.Source); got != string(input) {
		t.Fatalf("formatted source:\n%s\nwant:\n%s", got, input)
	}
}

func TestFormatLabeledLoop(t *testing.T) {
	input := []byte(`package p

func F(values [][]int) {
	outer:
	for _, values := range values {
		for _, value := range values {
			if value == 0 {
				continue outer
			}
		}
	}
}
`)
	result, err := Format("label.go", input)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(result.Source); got != string(input) {
		t.Fatalf("formatted source:\n%s\nwant:\n%s", got, input)
	}
}

func TestFormatConsolidatesAndGroupsImports(t *testing.T) {
	input := []byte(`package p
import "github.com/acme/lib"
import "fmt"
var _ = fmt.Println
var _ = lib.Value
`)
	result, err := Format("imports.go", input)
	if err != nil {
		t.Fatal(err)
	}
	want := "import (\n\t\"fmt\"\n\n\t\"github.com/acme/lib\"\n)"
	if !strings.Contains(string(result.Source), want) {
		t.Fatalf("imports not consolidated and grouped:\n%s", result.Source)
	}
}

func TestFormatCompositeLiteralComments(t *testing.T) {
	input := []byte(`package p

var values = map[string]string{
	"key": "value", // retain this comment
}
`)
	result, err := Format("composite_comments.go", input)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(result.Source); got != string(input) {
		t.Fatalf("formatted source:\n%s\nwant:\n%s", got, input)
	}
}

func TestFormatPreservesFreeFloatingCommentSeparation(t *testing.T) {
	input := []byte(`package p

func A() {}

// This is context, not documentation for B.

func B() {}
`)
	result, err := Format("free_comment.go", input)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(result.Source); got != string(input) {
		t.Fatalf("formatted source:\n%s\nwant:\n%s", got, input)
	}
}

func FuzzFormatRoundTrip(f *testing.F) {
	seeds := []string{
		"package p\n",
		"package p\nfunc F(a, b int) int { return a + b }\n",
		"package p\ntype T struct { Name string; Values []int }\n",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if !strings.Contains(source, "package ") {
			return
		}
		result, err := Format("fuzz.go", []byte(source))
		if err != nil {
			return
		}
		second, err := Format("fuzz.go", result.Source)
		if err != nil {
			t.Fatal(err)
		}
		if string(result.Source) != string(second.Source) {
			t.Fatal("formatter is not idempotent")
		}
	})
}

func BenchmarkFormat(b *testing.B) {
	source := []byte("package p\nfunc F(a, b int) int { if a > b { return a }; return b }\n")
	b.ReportAllocs()
	for range b.N {
		if _, err := Format("bench.go", source); err != nil {
			b.Fatal(err)
		}
	}
}
