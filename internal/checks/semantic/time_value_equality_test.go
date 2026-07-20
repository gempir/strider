package semantic

import (
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
