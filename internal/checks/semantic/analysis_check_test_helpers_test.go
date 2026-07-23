package semantic

import (
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func runStandaloneAnalysisCheck(t *testing.T, root string, check Check) []diagnostic.Diagnostic {
	t.Helper()
	meta := check.Meta()
	registry := &Registry{
		checks: []Check{
			check,
		},
		settings: map[string]configuredCheck{
			meta.Code: {
				severity: meta.DefaultSeverity,
			},
		},
	}
	diagnostics, err := Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	return diagnostics
}
