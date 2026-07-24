package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	diagnosticmodel "github.com/gempir/strider/internal/diagnostic"
)

func TestDigestNormalizesLineEndings(t *testing.T) {
	unix := digest(1, []byte("one\ntwo\n"), []byte("warning\n"))
	windows := digest(1, []byte("one\r\ntwo\r\n"), []byte("warning\r\n"))
	if unix != windows {
		t.Fatalf("line endings changed digest: %s != %s", unix, windows)
	}
	if unix == digest(0, []byte("one\ntwo\n"), []byte("warning\n")) {
		t.Fatal("exit code did not change digest")
	}
}

func TestWriteProjectReportLimitsDetailedDiagnostics(t *testing.T) {
	root := t.TempDir()
	diagnostics := make([]diagnosticmodel.Diagnostic, projectReportDiagnosticLimit+1)
	for index := range diagnostics {
		diagnostics[index] = diagnosticmodel.Diagnostic{
			Code:    "example",
			Message: "finding",
		}
	}
	result := projectResult{
		Name: "example",
		Operations: []operationResult{
			{
				Name:        "check",
				Diagnostics: diagnostics,
			},
		},
	}
	if err := writeProjectReport(root, result, ""); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(root + "/example/index.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(contents)
	if got := strings.Count(page, `<details class="diagnostic"`); got != projectReportDiagnosticLimit {
		t.Fatalf("rendered %d diagnostic details, want %d", got, projectReportDiagnosticLimit)
	}
	if !strings.Contains(page, "Showing 1000 of 1001 detailed findings") {
		t.Fatal("project report did not preserve the full diagnostic total")
	}
}

func TestNonEmptyLines(t *testing.T) {
	if got := nonEmptyLines("one\n\n  \ntwo\n"); got != 2 {
		t.Fatalf("nonEmptyLines = %d, want 2", got)
	}
}

func TestWriteProjectReportIncludesOperationTimings(t *testing.T) {
	root := t.TempDir()
	result := projectResult{
		Name: "example",
		Operations: []operationResult{
			{
				Name:       "format",
				DurationMS: 14,
			},
			{
				Name:       "check",
				DurationMS: 1190,
			},
		},
	}
	if err := writeProjectReport(root, result, ""); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(root + "/example/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{
		"format",
		"14 <small>ms</small>",
		"1190 <small>ms</small>",
	} {
		if !strings.Contains(string(contents), wanted) {
			t.Fatalf("project report missing timing %q", wanted)
		}
	}
}

func TestManifestRequiresElevenPinnedProjectsAndBudgets(t *testing.T) {
	path := t.TempDir() + "/projects.json"
	projects := make([]string, 11)
	for index := range projects {
		projects[index] = `{"name":"project-` + string(rune('a'+index)) + `","repository":"https://example.com/project.git","revision":"` + strings.Repeat("a", 40) + `","budgets_ms":{"format":1,"check":1}}`
	}
	contents := `{"version":1,"projects":[` + strings.Join(projects, ",") + `]}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readManifest(path); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
}

func TestWriteHomepageStatsExportsMeasuredDurations(t *testing.T) {
	path := t.TempDir() + "/homepage.json"
	results := report{
		Projects: []projectResult{
			{
				Name:     "sftpgo",
				Revision: strings.Repeat("a", 40),
				Operations: []operationResult{
					{
						Name:       "format",
						DurationMS: 458,
					},
					{
						Name:       "check",
						DurationMS: 5700,
					},
				},
			},
		},
	}
	if err := writeHomepageStats(path, results, "sftpgo"); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var stats homepageStats
	if err := json.Unmarshal(contents, &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Project != "sftpgo" || stats.FormatMS != 458 || stats.CheckMS != 5700 {
		t.Fatalf("unexpected homepage stats: %+v", stats)
	}
}

func TestBenchmarkVariantsKeepColdGoCacheOutOfFormat(t *testing.T) {
	options := options{
		schedulerModes:    "fixed,native",
		striderCacheModes: "cold,warm",
		goCacheModes:      "cold,warm",
	}
	assertBenchmarkVariantCount(t, "format", options, 4)
	assertBenchmarkVariantCount(t, "check", options, 8)
	assertBenchmarkVariantCount(t, "check-file-local", options, 4)
}

func assertBenchmarkVariantCount(t *testing.T, operation string, options options, want int) {
	t.Helper()

	variants, err := benchmarkVariants(operation, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(variants) != want {
		t.Fatalf("%s variants = %d, want %d", operation, len(variants), want)
	}
	if operation != "format" {
		return
	}
	for _, variant := range variants {
		if variant.goBuildCache != "warm" {
			t.Fatalf("format variant has Go cache mode %q", variant.goBuildCache)
		}
	}
}

func TestPercentileUsesNearestRank(t *testing.T) {
	samples := make([]sampleResult, 7)
	for index := range samples {
		samples[index].DurationNS = int64(index+1) * int64(time.Millisecond)
	}
	value := func(sample sampleResult) int64 {
		return sample.DurationNS / int64(time.Millisecond)
	}
	if got := medianInt64(samples, value); got != 4 {
		t.Fatalf("median = %d, want 4", got)
	}
	if got := percentileInt64(samples, 95, value); got != 7 {
		t.Fatalf("p95 = %d, want 7", got)
	}
}

func TestResetBenchmarkStateRejectsFilesystemRoot(t *testing.T) {
	if err := resetBenchmarkState(string(os.PathSeparator)); err == nil {
		t.Fatal("filesystem root was accepted as benchmark state")
	}
}

func TestCleanupBenchmarkCachesPreservesTelemetry(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{
		root + "/go-cache/entry",
		root + "/strider-cache/entry",
		root + "/telemetry/sample-1.json",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cleanupBenchmarkCaches(root)

	for _, path := range []string{
		root + "/go-cache",
		root + "/strider-cache",
	} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("cache path %s still exists: %v", path, err)
		}
	}
	if _, err := os.Stat(root + "/telemetry/sample-1.json"); err != nil {
		t.Fatalf("telemetry was removed: %v", err)
	}
}
