//strider:ignore-file cyclomatic-complexity
package resultcache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func TestRoundTripMaterializesInvocationData(t *testing.T) {
	cache, err := Open(Options{
		Directory: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	key := cache.Key([]byte("source"), []byte("configuration"))
	entry := Entry{
		FormatKnown:   true,
		FormatChanged: true,
		Findings: []Finding{
			{
				Code:    "example",
				Message: "message",
				Start:   10,
				End:     15,
				Fixes: []diagnostic.Fix{
					{
						Message: "fix",
						Safety:  diagnostic.Safe,
					},
				},
			},
		},
	}
	if err := cache.Put(key, entry); err != nil {
		t.Fatal(err)
	}
	loaded, ok := cache.Get(key)
	if !ok {
		t.Fatal("cache miss")
	}
	diagnostics := Materialize("relative.go", []byte("package p\nvar value = 1\n"), loaded.Findings, func(string) diagnostic.Severity {
		return diagnostic.SeverityWarning
	})
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
	item := diagnostics[0]
	if item.File != "relative.go" || item.Start.Filename != "relative.go" || item.Start.Line != 2 || item.Severity != diagnostic.SeverityWarning {
		t.Fatalf("materialized diagnostic = %+v", item)
	}
	contents, err := os.ReadFile(cache.entryPath(key))
	if err != nil {
		t.Fatal(err)
	}
	if containsAny(string(contents), "absolute.go", "relative.go", "warning") {
		t.Fatalf("persisted invocation-specific data: %s", contents)
	}
}

func TestCorruptAndSchemaMismatchEntriesAreSafeMisses(t *testing.T) {
	cache, err := Open(Options{
		Directory: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	key := cache.Key([]byte("corrupt"))
	path := cache.entryPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.Get(key); ok {
		t.Fatal("corrupt entry was a hit")
	}
	mismatch, err := json.Marshal(envelope{
		SchemaVersion: SchemaVersion + 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, mismatch, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.Get(key); ok {
		t.Fatal("schema mismatch was a hit")
	}
}

func TestConcurrentWritersPublishCompleteEntry(t *testing.T) {
	cache, err := Open(Options{
		Directory: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	key := cache.Key([]byte("shared"))
	var group sync.WaitGroup
	for range 16 {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := cache.Put(key, Entry{
				FormatKnown:   true,
				FormatChanged: true,
			}); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	entry, ok := cache.Get(key)
	if !ok || !entry.FormatKnown || !entry.FormatChanged {
		t.Fatalf("entry = %+v, hit = %t", entry, ok)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(cache.entryPath(key)), ".entry-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary entries remain: %v", matches)
	}
}

func TestByteBoundEvictsDeterministicallyAndClearRemovesEntries(t *testing.T) {
	cache, err := Open(Options{
		Directory: t.TempDir(),
		MaxBytes:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	key := cache.Key([]byte("oversize"))
	if err := cache.Put(key, Entry{
		FormatKnown: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.Get(key); ok {
		t.Fatal("oversized entry was not evicted")
	}
	cache.maxBytes = 1 << 20
	if err := cache.Put(key, Entry{
		FormatKnown: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := cache.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.Get(key); ok {
		t.Fatal("clear retained an entry")
	}
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
