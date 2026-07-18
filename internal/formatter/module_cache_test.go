package formatter

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestModulePathCacheReusesPositiveLookupConcurrently(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module example.com/first\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "nested", "package", "file.go")
	cache := &modulePathCache{}
	if got := cache.find(filename); got != "example.com/first" {
		t.Fatalf("module path = %q, want example.com/first", got)
	}
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module example.com/changed\n"),
		0o600,
	); err != nil {
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
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module example.com/created-later\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if got := cache.find(filename); got != "" {
		t.Fatalf("cached module path = %q, want empty", got)
	}
}
