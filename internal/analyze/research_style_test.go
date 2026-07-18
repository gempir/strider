package analyze

import "testing"

func TestResearchStyleRules(t *testing.T) {
	root := analysisModule(
		t,
		`// Package sample demonstrates checks
package sample

import (
	"crypto/md5"
	"net/http"
)

// ParseFailure describes a parse failure
type ParseFailure struct{}
func (ParseFailure) Error() string { return "parse" }

// Exported lacks punctuation
func Exported() {
	_, _, _, value := results()
	_ = value
	// TODO: replace the legacy digest.
	_ = md5.Sum(nil)
	_, _ = http.NewRequest("GET", "https://example.com", nil)
}

func results() (int, int, int, int) { return 0, 0, 0, 0 }
`,
	)
	codes := []string{
		"excessive-blank-identifiers",
		"task-comment",
		"doc-comment-period",
		"error-type-naming",
		"standard-http-method-constant",
		"weak-cryptography",
	}
	registry, err := NewRegistry(codes)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	counts := make(map[string]int)
	for _, item := range diagnostics {
		counts[item.Code]++
	}
	for _, code := range codes {
		if counts[code] == 0 {
			t.Errorf("%s produced no diagnostic: %#v", code, diagnostics)
		}
	}
}

func TestResearchStyleRulesAcceptGoodForms(t *testing.T) {
	root := analysisModule(
		t,
		`// Package sample demonstrates accepted forms.
package sample

import (
	"crypto/sha256"
	"net/http"
)

// ParseError describes a parse failure.
type ParseError struct{}
func (ParseError) Error() string { return "parse" }

// Exported has complete documentation.
func Exported() {
	first, second, third, value := results()
	use(first, second, third, value)
	_ = sha256.Sum256(nil)
	// This bug was fixed before release.
	_, _ = http.NewRequest(http.MethodGet, "https://example.com", nil)
}

type text string
type failure struct{}
func (failure) Error() text { return "not the built-in string result" }

func results() (int, int, int, int) { return 0, 0, 0, 0 }
func use(...int) {}
`,
	)
	codes := []string{
		"excessive-blank-identifiers",
		"task-comment",
		"doc-comment-period",
		"error-type-naming",
		"standard-http-method-constant",
		"weak-cryptography",
	}
	registry, err := NewRegistry(codes)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
}

func TestDocCommentPeriodDeduplicatesGroupedDeclarationDocs(t *testing.T) {
	root := analysisModule(
		t,
		`// Package sample demonstrates grouped declarations.
package sample

// Values are exported constants
const (
	First = 1
	Second = 2
)
`,
	)
	registry, err := NewRegistry([]string{"doc-comment-period"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf(
			"got %d diagnostics, want one for the shared comment: %#v",
			len(diagnostics),
			diagnostics,
		)
	}
}
