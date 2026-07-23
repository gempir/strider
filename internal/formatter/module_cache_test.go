package formatter

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestFormatterSessionKeepsModuleImportsSeparate(t *testing.T) {
	root := t.TempDir()
	session := NewFormatter()
	for _, test := range []struct {
		name   string
		module string
	}{
		{
			name:   "first",
			module: "example.com/first",
		},
		{
			name:   "second",
			module: "example.org/second",
		},
	} {
		t.Run(
			test.name,
			func(t *testing.T) {
				moduleRoot := filepath.Join(root, test.name)
				if err := os.MkdirAll(moduleRoot, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(moduleRoot, "go.mod"), []byte("module "+test.module+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				source := []byte("package sample\nimport (\n\"" + test.module + "/local\"\n\"fmt\"\n\"example.net/external\"\n)\nvar _ = fmt.Println\n")
				result, err := session.FormatWithOptions(filepath.Join(moduleRoot, "main.go"), source, DefaultOptions())
				if err != nil {
					t.Fatal(err)
				}
				wantGroups := "\"fmt\"\n\n\t\"example.net/external\"\n\n\t\"" + test.module + "/local\""
				if !strings.Contains(string(result.Source), wantGroups) {
					t.Fatalf("imports were not grouped for %s:\n%s", test.module, result.Source)
				}
			},
		)
	}
}

func TestModulePathCacheReusesPositiveLookupConcurrently(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "nested", "package", "file.go")
	cache := &modulePathCache{}
	if got := cache.find(filename); got != "example.com/first" {
		t.Fatalf("module path = %q, want example.com/first", got)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	results := make(chan string, 32)
	var group sync.WaitGroup
	for range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			results <- cache.find(filename)
		}()
	}
	group.Wait()
	close(results)
	for got := range results {
		if got != "example.com/first" {
			t.Fatalf("cached module path = %q, want example.com/first", got)
		}
	}
}

func TestModulePathCacheReusesNegativeLookup(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "nested", "file.go")
	cache := &modulePathCache{}
	if got := cache.find(filename); got != "" {
		t.Fatalf("module path = %q, want empty", got)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/created-later\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := cache.find(filename); got != "" {
		t.Fatalf("cached module path = %q, want empty", got)
	}
}

func TestModulePathInParsesQuotedDirective(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("// module ignored.example\nmodule \"example.com/quoted\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, found := modulePathIn(root)
	if !found || path != "example.com/quoted" {
		t.Fatalf("modulePathIn = %q, %t; want parsed quoted module path", path, found)
	}
}
