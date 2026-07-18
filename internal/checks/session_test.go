package checks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/workspace"
)

func TestSessionReusesAndInvalidatesConcreteGeneration(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "sample.go")
	if err := os.WriteFile(filename, []byte("package sample\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(RegistryOptions{Only: []string{"no-init"}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewSession(registry, RunOptions{}, SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cache := workspace.NewCache(workspace.CacheOptions{MaxEntries: 8, MaxBytes: 1 << 20})

	first := runCachedSession(t, cache, session, filename)
	if len(first.Diagnostics) != 1 || first.Diagnostics[0].Code != "no-init" {
		t.Fatalf("first diagnostics = %#v", first.Diagnostics)
	}
	first.Diagnostics[0].Code = "mutated-by-caller"
	second := runCachedSession(t, cache, session, filename)
	if len(second.Diagnostics) != 1 || second.Diagnostics[0].Code != "no-init" {
		t.Fatalf("cached diagnostics were not owned: %#v", second.Diagnostics)
	}
	stats := session.Stats()
	if stats.ConcreteHits != 1 || stats.ConcreteMisses != 1 {
		t.Fatalf("unchanged generation stats = %#v", stats)
	}

	// Keep the byte length stable to prove invalidation uses content identity,
	// not timestamps or file size.
	if err := os.WriteFile(filename, []byte("package sample\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	third := runCachedSession(t, cache, session, filename)
	if len(third.Diagnostics) != 0 {
		t.Fatalf("changed diagnostics = %#v", third.Diagnostics)
	}
	stats = session.Stats()
	if stats.ConcreteHits != 1 || stats.ConcreteMisses != 2 {
		t.Fatalf("changed generation stats = %#v", stats)
	}
}

func TestSessionOwnsFormattingCandidates(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "sample.go")
	if err := os.WriteFile(filename, []byte("package sample\nfunc F( ){return}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(RegistryOptions{Only: []string{"format"}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewSession(
		registry,
		RunOptions{Formatter: formatter.DefaultOptions(), CollectCandidates: true},
		SessionOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	cache := workspace.NewCache(workspace.CacheOptions{MaxEntries: 8, MaxBytes: 1 << 20})
	first := runCachedSession(t, cache, session, filename)
	candidate := first.Candidates[filename]
	if len(candidate.Source) == 0 {
		t.Fatal("missing formatting candidate")
	}
	candidate.Source[0] = 'X'
	first.Candidates[filename] = candidate
	second := runCachedSession(t, cache, session, filename)
	if second.Candidates[filename].Source[0] != 'p' {
		t.Fatalf("cached candidate was not owned: %q", second.Candidates[filename].Source)
	}
}

func TestSessionInvalidatesFormattingWhenModulePathChanges(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "sample.go")
	if err := os.WriteFile(
		filename,
		[]byte("package sample\nimport (\n\t\"fmt\"\n\t\"example.com/one/pkg\"\n)\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(directory, "go.mod")
	if err := os.WriteFile(module, []byte("module example.com/one\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(RegistryOptions{Only: []string{"format"}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewSession(registry, RunOptions{}, SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cache := workspace.NewCache(workspace.CacheOptions{MaxEntries: 8, MaxBytes: 1 << 20})
	first := runCachedSession(t, cache, session, filename)
	if len(first.Candidates) != 0 {
		t.Fatalf("read-only session retained formatting candidates: %#v", first.Candidates)
	}
	_ = runCachedSession(t, cache, session, filename)
	if stats := session.Stats(); stats.ConcreteHits != 1 || stats.ConcreteMisses != 1 {
		t.Fatalf("warm module stats = %#v", stats)
	}
	if err := os.WriteFile(module, []byte("module example.com/two\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = runCachedSession(t, cache, session, filename)
	if stats := session.Stats(); stats.ConcreteHits != 1 || stats.ConcreteMisses != 2 {
		t.Fatalf("changed module stats = %#v", stats)
	}
}

func runCachedSession(t testingTB, cache *workspace.Cache, session *Session, filename string) Result {
	t.Helper()
	shared, err := cache.Open([]string{filename}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := session.Run(shared)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func BenchmarkSessionUnchangedConcreteGeneration(b *testing.B) {
	directory := b.TempDir()
	filename := filepath.Join(directory, "sample.go")
	if err := os.WriteFile(
		filename,
		[]byte("package sample\nfunc init() { println(\"x\") }\n"),
		0o600,
	); err != nil {
		b.Fatal(err)
	}
	registry, err := NewRegistry(RegistryOptions{Only: []string{"no-init"}})
	if err != nil {
		b.Fatal(err)
	}
	session, err := NewSession(registry, RunOptions{}, SessionOptions{})
	if err != nil {
		b.Fatal(err)
	}
	cache := workspace.NewCache(workspace.CacheOptions{MaxEntries: 8, MaxBytes: 1 << 20})
	if result := runCachedSession(b, cache, session, filename); len(result.Diagnostics) != 1 {
		b.Fatalf("warm-up diagnostics = %#v", result.Diagnostics)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if result := runCachedSession(b, cache, session, filename); len(result.Diagnostics) != 1 {
			b.Fatalf("diagnostics = %#v", result.Diagnostics)
		}
	}
}

type testingTB interface {
	Helper()
	Fatal(...any)
}
