//strider:ignore-file cognitive-complexity,excessive-blank-identifiers
package semantic

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func assertDiagnosticGolden(t *testing.T, diagnostics []diagnostic.Diagnostic) {
	t.Helper()
	var got strings.Builder
	for _, item := range diagnostics {
		fmt.Fprintf(
			&got,
			"%s:%d:%d-%d:%d %s\n",
			filepath.Base(filepath.FromSlash(item.Start.Filename)),
			item.Start.Line,
			item.Start.Column,
			item.End.Line,
			item.End.Column,
			item.Code,
		)
	}
	_, helperFile, _, _ := runtime.Caller(0)
	goldenName := strings.NewReplacer("/", "__", " ", "_").Replace(t.Name()) + ".golden"
	goldenPath := filepath.Join(filepath.Dir(helperFile), "testdata", "diagnostics", goldenName)
	if os.Getenv("STRIDER_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, []byte(got.String()), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		t.Fatalf("diagnostic positions differ from %s\ngot:\n%swant:\n%s", filepath.Base(goldenPath), got.String(), want)
	}
}

func analysisModule(t *testing.T, source string) string {
	return analysisModuleVersion(t, "1.26", source)
}

func analysisModuleVersion(t *testing.T, goVersion, source string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/analysis\n\ngo "+goVersion+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}
