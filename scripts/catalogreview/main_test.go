package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/checks"
	"github.com/gempir/strider/internal/diagnostic"
)

func TestCommittedCatalogReviewMatchesRetainedInventory(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	contents, err := os.ReadFile(filepath.Join(root, "benchmarks", "catalog-review.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reviewed report
	if err := json.Unmarshal(contents, &reviewed); err != nil {
		t.Fatal(err)
	}
	registry, err := checks.NewRegistry(checks.RegistryOptions{
		MinimumSeverity: diagnostic.SeverityNone,
		Root:            root,
		Directory:       root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(reviewed.Checks), len(registry.Checks()); got != want {
		t.Fatalf("reviewed %d retained checks; catalog has %d", got, want)
	}
	byCode := make(map[string]checkReview, len(reviewed.Checks))
	for _, entry := range reviewed.Checks {
		byCode[entry.Code] = entry
		if entry.Stage == "" || entry.ImplementationLOC <= 0 || entry.ImplementationUnit == "" {
			t.Errorf("%s has incomplete implementation evidence: %+v", entry.Code, entry)
		}
		if entry.SignalRationale == "" || entry.DecisionRationale == "" || entry.FalsePositiveBoundary == "" {
			t.Errorf("%s has incomplete product review", entry.Code)
		}
		if entry.AnalyzerOverlap == "" || entry.FixAvailability == "" || entry.ExampleEvidence == "" {
			t.Errorf("%s has incomplete analyzer, fix, or example evidence", entry.Code)
		}
		if strings.Contains(entry.SignalRationale, "Default: enabled") {
			t.Errorf("%s retains filler default metadata", entry.Code)
		}
	}
	for _, descriptor := range registry.Checks() {
		entry, found := byCode[descriptor.Meta().Code]
		if !found {
			t.Errorf("%s has no catalog review", descriptor.Meta().Code)
			continue
		}
		if entry.Engine != string(descriptor.Engine()) || entry.DefaultSeverity != string(descriptor.Meta().DefaultSeverity) {
			t.Errorf("%s review has stale engine or severity", descriptor.Meta().Code)
		}
	}
	for code, decision := range reviewedDecisions {
		entry, found := byCode[code]
		if !found {
			t.Errorf("explicitly reviewed check %s is not retained", code)
			continue
		}
		if entry.Decision != decision.action || entry.DecisionRationale != decision.rationale || entry.FalsePositiveBoundary != decision.boundary {
			t.Errorf("%s explicit review is stale", code)
		}
	}
	if got, want := len(reviewed.RemovedChecks), len(removedDecisions); got != want {
		t.Fatalf("reviewed %d removals; want %d", got, want)
	}
}

func TestLogicalLOCExcludesWhitespaceAndComments(t *testing.T) {
	contents := []byte("package p\n\n// comment\nvar value int\n/* block\ncomment */\nfunc use() {}\n")
	if got, want := logicalLOC(contents), 3; got != want {
		t.Fatalf("logicalLOC = %d, want %d", got, want)
	}
}
