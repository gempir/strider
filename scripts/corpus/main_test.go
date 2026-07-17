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

func TestManifestRequiresTenPinnedProjectsAndBudgets(t *testing.T) {
	path := t.TempDir() + "/projects.json"
	projects := make([]string, 10)
	for index := range projects {
		projects[index] = `{"name":"project-` + string(rune('a'+index)) + `","repository":"https://example.com/project.git","revision":"` + strings.Repeat("a", 40) + `","budgets_ms":{"format":1,"lint":1,"analyze":1}}`
	}
	contents := `{"version":1,"projects":[` + strings.Join(projects, ",") + `]}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readManifest(path); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
}
