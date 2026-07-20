package semantic

import (
	"strings"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func TestTimeValueEqualityOnlyReportsActualTimeValues(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "time"

type TimeAlias = time.Time
type localTime time.Time

func compare(left, right time.Time, alias TimeAlias, localLeft, localRight localTime, pointerLeft, pointerRight *time.Time) {
	_ = left == right
	_ = left != alias
	_ = localLeft == localRight
	_ = pointerLeft == pointerRight
	_ = left.Equal(right)
}
`,
	)
	registry, err := newRegistry([]string{
		"time-value-equality",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
	for _, item := range diagnostics {
		if item.Code != "time-value-equality" || item.Severity != diagnostic.SeverityWarning {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}

func TestWaitGroupGoForbiddenCallUsesResolvedMethodsAndBuiltins(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "sync"

type fakeGroup struct{}

func (fakeGroup) Go(func()) {}
func (fakeGroup) Done() {}

func check(group *sync.WaitGroup, fake fakeGroup) {
	group.Go(func() {
		panic("failed")
		recover()
		group.Done()
		fake.Done()
	})
	fake.Go(func() {
		panic("allowed by this check")
		recover()
		group.Done()
	})
	group.Go(func() {
		panic := func(any) {}
		recover := func() any { return nil }
		panic("shadowed")
		_ = recover()
	})
	group.Go(func() {
		deferred := func() { group.Done() }
		_ = deferred
	})
}
`,
	)
	registry, err := newRegistry([]string{
		"waitgroup-go-forbidden-call",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 3 {
		t.Fatalf("got %d diagnostics, want 3: %#v", len(diagnostics), diagnostics)
	}
	for _, item := range diagnostics {
		if item.Code != "waitgroup-go-forbidden-call" || item.Severity != diagnostic.SeverityError {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}

func TestRangeValueCaptureRespectsGoVersionAssignmentAndInvocation(t *testing.T) {
	tests := []struct {
		name      string
		goVersion string
		source    string
		want      int
	}{
		{
			name:      "legacy declaration is reused",
			goVersion: "1.21",
			source: `package sample

func capture(values []int) []func() {
	var callbacks []func()
	for _, value := range values {
		callbacks = append(callbacks, func() { _ = value })
	}
	return callbacks
}
`,
			want: 1,
		},
		{
			name:      "modern declaration is per iteration",
			goVersion: "1.22",
			source: `package sample

func capture(values []int) []func() {
	var callbacks []func()
	for _, value := range values {
		callbacks = append(callbacks, func() { _ = value })
	}
	return callbacks
}
`,
		},
		{
			name:      "assignment remains reused",
			goVersion: "1.26",
			source: `package sample

func capture(values []int) []func() {
	var value int
	var callbacks []func()
	for _, value = range values {
		callbacks = append(callbacks, func() { _ = value })
	}
	return callbacks
}
`,
			want: 1,
		},
		{
			name:      "synchronous invocation is safe",
			goVersion: "1.21",
			source: `package sample

func use(int) {}

func capture(values []int) {
	for _, value := range values {
		func() { use(value) }()
	}
}
`,
		},
		{
			name:      "go invocation can outlive iteration",
			goVersion: "1.21",
			source: `package sample

func use(int) {}

func capture(values []int) {
	for _, value := range values {
		go func() { use(value) }()
	}
}
`,
			want: 1,
		},
	}
	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				root := analysisModuleVersion(t, test.goVersion, test.source)
				registry,
					err := newRegistry([]string{
					"range-value-capture",
				})
				if err != nil {
					t.Fatal(err)
				}
				diagnostics,
					err := Run([]string{
					root,
				}, registry)
				if err != nil {
					t.Fatal(err)
				}
				if len(diagnostics) != test.want {
					t.Fatalf("got %d diagnostics, want %d: %#v", len(diagnostics), test.want, diagnostics)
				}
				for _, item := range diagnostics {
					if item.Code != "range-value-capture" || !strings.Contains(item.Message, "range variable") {
						t.Fatalf("unexpected diagnostic: %#v", item)
					}
				}
			},
		)
	}
}

func TestMovedSyntaxRuleMetadata(t *testing.T) {
	tests := []struct {
		rule     Rule
		severity diagnostic.Severity
	}{
		{
			timeValueEqualityRule{},
			diagnostic.SeverityWarning,
		},
		{
			waitGroupGoForbiddenCallRule{},
			diagnostic.SeverityError,
		},
		{
			rangeValueCaptureRule{},
			diagnostic.SeverityWarning,
		},
	}
	for _, test := range tests {
		meta := test.rule.Meta()
		if meta.DefaultSeverity != test.severity || meta.Summary == "" || meta.Explanation == "" || meta.GoodExample == "" || meta.BadExample == "" {
			t.Errorf("incomplete metadata for %s: %#v", meta.Code, meta)
		}
	}
}
