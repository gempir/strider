package main

import (
	"os"
	"strings"
	"testing"
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
			{Name: "format", DurationMS: 14},
			{Name: "lint", DurationMS: 287},
			{Name: "analyze", DurationMS: 903},
		},
	}
	if err := writeProjectReport(root, result, ""); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(root + "/example/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, wanted := range[]string{"format", "14 <small>ms</small>", "903 <small>ms</small>"} {
		if !strings.Contains(string(contents), wanted) {
			t.Fatalf("project report missing timing %q", wanted)
		}
	}
}

func TestManifestRequiresTwentySixPinnedProjectsAndBudgets(t *testing.T) {
	path := t.TempDir() + "/projects.json"
	projects := make([]string, 26)
	for index := range projects {
		projects[index] = `{"name":"project-` + string(rune('a' + index)) + `","repository":"https://example.com/project.git","revision":"` + strings.Repeat(
			"a",
			40,
		) + `","budgets_ms":{"format":1,"lint":1,"analyze":1}}`
	}
	contents := `{"version":1,"projects":[` + strings.Join(projects, ",") + `]}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readManifest(path); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
}
