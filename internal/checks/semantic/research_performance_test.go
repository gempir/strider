package semantic

import (
	"strings"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func TestResearchPerformanceRuleMetadata(t *testing.T) {
	tests := []struct {
		rule     Rule
		severity diagnostic.Severity
	}{
		{
			appendToSizedSliceRule{},
			diagnostic.SeverityWarning,
		},
		{
			redundantConversionRule{},
			diagnostic.SeverityWarning,
		},
		{
			slicePreallocationRule{},
			diagnostic.SeverityWarning,
		},
		{
			inefficientSprintfRule{},
			diagnostic.SeverityWarning,
		},
		{
			interfaceMethodLimitRule{},
			diagnostic.SeverityWarning,
		},
		{
			constructorInterfaceReturnRule{},
			diagnostic.SeverityWarning,
		},
		{
			slogArgumentShapeRule{},
			diagnostic.SeverityWarning,
		},
		{
			externalCallInLoopRule{},
			diagnostic.SeverityWarning,
		},
	}
	for _, test := range tests {
		meta := test.rule.Meta()
		t.Run(
			meta.Code,
			func(t *testing.T) {
				if meta.DefaultSeverity != test.severity {
					t.Fatalf("default severity = %q, want %q", meta.DefaultSeverity, test.severity)
				}
				if meta.Summary == "" || meta.Explanation == "" || meta.GoodExample == "" || meta.BadExample == "" {
					t.Fatalf("incomplete metadata: %#v", meta)
				}
			},
		)
	}
}

func TestResearchPerformanceRulesReportConservativeCases(t *testing.T) {
	tests := []struct {
		code       string
		source     string
		want       int
		messageHas string
	}{
		{
			code: "append-to-sized-slice",
			source: `package sample

var packageValues = make([]int, 2)

func check() {
	values := make([]int, 3)
	values = append(values, 1)
	values = append(values, 2)
	loopValues := make([]int, 2)
	for value := range 2 {
		loopValues = append(loopValues, value)
	}

	reset := make([]int, 3)
	reset = reset[:0]
	reset = append(reset, 1)

	capacityOnly := make([]int, 0, 3)
	capacityOnly = append(capacityOnly, 1)
	packageValues = append(packageValues, 1)
}
`,
			want:       2,
			messageHas: "positive length",
		},
		{
			code: "redundant-conversion",
			source: `package sample

type UserID int

func check(existing UserID, raw int) {
	_ = UserID(existing)
	_ = UserID(raw)
}
`,
			want:       1,
			messageHas: "identical type",
		},
		{
			code: "slice-preallocation",
			source: `package sample

func collect(source []int) []int {
	var result []int
	for _, value := range source {
		result = append(result, value)
	}
	return result
}

func alreadySized(source []int) []int {
	result := make([]int, 0, len(source))
	for _, value := range source {
		result = append(result, value)
	}
	return result
}

func conditional(source []int) []int {
	var result []int
	for _, value := range source {
		if value > 0 {
			result = append(result, value)
		}
	}
	return result
}

func resetEachIteration(source []int) []int {
	var result []int
	for _, value := range source {
		result = nil
		result = append(result, value)
	}
	return result
}

func rangeOverResult() []int {
	var result []int
	for _, value := range result {
		result = append(result, value)
	}
	return result
}
`,
			want:       1,
			messageHas: "preallocate",
		},
		{
			code: "inefficient-sprintf",
			source: `package sample

import "fmt"

type labeled string
type customInt int

func (l labeled) String() string { return "label:" + string(l) }
func (customInt) Format(fmt.State, rune) {}

func check(number int, text string, custom labeled) {
	_ = fmt.Sprintf("%d", number)
	_ = fmt.Sprintf("%s", text)
	_ = fmt.Sprintf("%v", number)
	_ = fmt.Sprintf("%f", float64(number))
	_ = fmt.Sprintf("%s", custom)
	_ = fmt.Sprintf("%d", customInt(number))
}
`,
			want:       3,
			messageHas: "unnecessary",
		},
		{
			code: "interface-method-limit",
			source: `package sample

type focused interface {
	A(); B(); C(); D(); E(); F(); G(); H(); I(); J()
}

type bloated interface {
	A(); B(); C(); D(); E(); F(); G(); H(); I(); J(); K()
}
`,
			want:       1,
			messageHas: "11 methods",
		},
		{
			code: "constructor-interface-return",
			source: `package sample

type Store interface { Get(string) string }

type memoryStore struct{}

func (*memoryStore) Get(key string) string { return key }

func NewStore() Store { return &memoryStore{} }
func BuildStore() Store { return &memoryStore{} }
func NewAny() any { return &memoryStore{} }
`,
			want:       1,
			messageHas: "concrete result",
		},
		{
			code: "slog-argument-shape",
			source: `package sample

import "log/slog"

type key string

func check(value any) {
	slog.Info("odd", "key")
	slog.Info("non-string", 42, value)
	slog.Info("mixed", slog.String("key", "value"), "other", value)
	slog.Info("named string", key("key"), value)
	slog.Info("pairs", "key", value)
	slog.Info("attrs", slog.String("key", "value"), slog.Any("other", value))
}
`,
			want:       4,
			messageHas: "slog",
		},
		{
			code: "external-call-in-loop",
			source: `package sample

import (
	"database/sql"
	"net/http"
)

func check(db *sql.DB, client *http.Client, request *http.Request, ids []int) {
	for range ids {
		db.Query("SELECT value FROM records")
		client.Do(request)
	}

	db.Exec("DELETE FROM records")
	for range ids {
		_ = func() {
			db.Query("SELECT nested FROM records")
		}
		go db.Exec("UPDATE records SET seen = TRUE")
	}
}
`,
			want:       2,
			messageHas: "inside a loop",
		},
	}

	for _, test := range tests {
		t.Run(
			test.code,
			func(t *testing.T) {
				root := analysisModule(t, test.source)
				registry,
					err := NewRegistry([]string{
					test.code,
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
					if item.Code != test.code || !strings.Contains(item.Message, test.messageHas) {
						t.Fatalf("unexpected diagnostic: %#v", item)
					}
				}
			},
		)
	}
}

func TestResearchPerformanceRuleStages(t *testing.T) {
	ssaRules := map[string]bool{
		"append-to-sized-slice": true,
		"external-call-in-loop": true,
	}
	for _, rule := range []Rule{
		appendToSizedSliceRule{},
		redundantConversionRule{},
		slicePreallocationRule{},
		inefficientSprintfRule{},
		interfaceMethodLimitRule{},
		constructorInterfaceReturnRule{},
		slogArgumentShapeRule{},
		externalCallInLoopRule{},
	} {
		code := rule.Meta().Code
		if got := UsesSSA(code); got != ssaRules[code] {
			t.Errorf("UsesSSA(%q) = %v, want %v", code, got, ssaRules[code])
		}
	}
}
