//strider:ignore-file cognitive-complexity
package semantic

import "testing"

const badStyleCheckSource = `// Package sample demonstrates checks
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
`

const goodStyleCheckSource = `// Package sample demonstrates accepted forms.
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
`

func assertStyleCheck(t *testing.T, code string) {
	t.Helper()
	badRoot := analysisModule(t, badStyleCheckSource)
	registry, err := newRegistry([]string{
		code,
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		badRoot,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 {
		t.Fatalf("%s produced no diagnostic", code)
	}
	for _, item := range diagnostics {
		if item.Code != code {
			t.Fatalf("%s run reported %s", code, item.Code)
		}
	}

	goodRoot := analysisModule(t, goodStyleCheckSource)
	diagnostics, err = Run([]string{
		goodRoot,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("%s reported accepted forms: %#v", code, diagnostics)
	}
}
