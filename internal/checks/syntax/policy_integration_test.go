package syntax

import (
	"reflect"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

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
