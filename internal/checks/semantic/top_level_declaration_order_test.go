package semantic

import (
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func TestTopLevelDeclarationOrderReportsOneRegressionPerFile(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

const first = 1

var current item

type item struct{}

func init() { current = item{} }
func use() item { return current }

type later struct{}
const late = 2
`,
	)
	diagnostics := runStandaloneAnalysisCheck(t, root, topLevelDeclarationOrderCheck{})
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
	if diagnostics[0].Code != "top-level-declaration-order" || diagnostics[0].Severity != diagnostic.SeverityWarning {
		t.Fatalf("unexpected diagnostic: %#v", diagnostics[0])
	}
}

func TestTopLevelDeclarationOrderAcceptsConstVarTypeAndFunctions(t *testing.T) {
	root := analysisModule(t, `package sample

const first = 1

var current item

type item struct{}

func init() { current = item{} }
func use() item { return current }
`)
	diagnostics := runStandaloneAnalysisCheck(t, root, topLevelDeclarationOrderCheck{})
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
}
