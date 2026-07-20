package semantic

import (
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func TestDiscardedErrorResultTracksTypedResultPositions(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type writer interface { Write([]byte) (int, error) }

func pair() (int, error) { return 0, nil }
func errorFirst() (error, int) { return nil, 0 }
func one() error { return nil }
func noError() int { return 0 }

var _ = one()

func check(w writer) {
	pair()
	value, _ := pair()
	_, value = errorFirst()
	_ = one()
	_, value = one(), noError()
	w.Write(nil)
	noError()
	_, err := pair()
	err, _ = errorFirst()
	_ = value
	_ = err
}
`,
	)
	diagnostics := runStandaloneAnalysisCheck(t, root, discardedErrorResultCheck{})
	assertDiagnosticGolden(t, diagnostics)
	for _, item := range diagnostics {
		if item.Code != "discarded-error-result" || item.Severity != diagnostic.SeverityError {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}

func TestDiscardedErrorResultAllowsInfallibleWriters(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"bytes"
	"fmt"
	"strings"
)

func check() {
	var buffer bytes.Buffer
	var builder strings.Builder
	fmt.Fprintln(&buffer, "safe")
	buffer.WriteString("safe")
	builder.WriteString("safe")
}
`,
	)
	diagnostics := runStandaloneAnalysisCheck(t, root, discardedErrorResultCheck{})
	if len(diagnostics) != 0 {
		t.Fatalf("infallible writes reported: %#v", diagnostics)
	}
}
