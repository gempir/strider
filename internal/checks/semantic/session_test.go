package semantic

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
)

func TestSessionCachesDeepCopiedWholeTargetResult(t *testing.T) {
	var runs atomic.Int32
	want := diagnostic.Diagnostic{
		Code:    "check",
		Message: "original",
		Notes: []diagnostic.Note{
			{
				Message: "note",
			},
		},
		Fixes: []diagnostic.Fix{
			{
				Message: "fix",
				Edits: []diagnostic.TextEdit{
					{
						NewText: "replacement",
					},
				},
			},
		},
	}
	session := newSession(
		SessionOptions{
			MaxEntries: 2,
			MaxBytes:   1 << 20,
		},
		labelFingerprint,
		func([]string, *Registry) ([]diagnostic.Diagnostic, error) {
			runs.Add(1)
			return []diagnostic.Diagnostic{
					want,
				},
				nil
		},
	)
	registry := &Registry{}
	first, err := session.Run([]string{
		"target",
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	first[0].Message = "mutated"
	first[0].Notes[0].Message = "mutated"
	first[0].Fixes[0].Edits[0].NewText = "mutated"
	second, err := session.Run([]string{
		"target",
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if runs.Load() != 1 {
		t.Fatalf("runner calls = %d, want 1", runs.Load())
	}
	if second[0].Message != want.Message || second[0].Notes[0].Message != "note" || second[0].Fixes[0].Edits[0].NewText != "replacement" {
		t.Fatalf("cached result was aliased: %#v", second)
	}
	stats := session.Stats()
	if stats.Hits != 1 || stats.Misses != 1 || stats.Entries != 1 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestSessionConcurrentRunCoalescesWholeTarget(t *testing.T) {
	var runs atomic.Int32
	session := newSession(
		SessionOptions{
			MaxEntries: 2,
			MaxBytes:   1 << 20,
		},
		labelFingerprint,
		func([]string, *Registry) ([]diagnostic.Diagnostic, error) {
			runs.Add(1)
			return []diagnostic.Diagnostic{
					{
						Code: "check",
					},
				},
				nil
		},
	)
	registry := &Registry{}
	errors := make(chan error, 16)
	var group sync.WaitGroup
	for range 16 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := session.Run([]string{
				"target",
			}, registry)
			errors <- err
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Error(err)
		}
	}
	if runs.Load() != 1 {
		t.Fatalf("runner calls = %d, want 1", runs.Load())
	}
	stats := session.Stats()
	if stats.Hits != 15 || stats.Misses != 1 {
		t.Fatalf("unexpected concurrent stats: %#v", stats)
	}
}

func TestSessionRetriesGenerationChangedDuringAnalysis(t *testing.T) {
	var generation atomic.Int32
	var runs atomic.Int32
	session := newSession(
		SessionOptions{
			MaxEntries: 2,
			MaxBytes:   1 << 20,
		},
		func([]string, *Registry) (analysisCacheKey, error) {
			return sha256.Sum256([]byte(fmt.Sprint(generation.Load()))),
				nil
		},
		func([]string, *Registry) ([]diagnostic.Diagnostic, error) {
			current := runs.Add(1)
			if current == 1 {
				generation.Store(1)
			}
			return []diagnostic.Diagnostic{
					{
						Code: fmt.Sprint(current),
					},
				},
				nil
		},
	)
	diagnostics, err := session.Run([]string{
		"target",
	}, &Registry{})
	if err != nil {
		t.Fatal(err)
	}
	if runs.Load() != 2 || len(diagnostics) != 1 || diagnostics[0].Code != "2" {
		t.Fatalf("unstable result was returned or cached: runs %d, diagnostics %#v", runs.Load(), diagnostics)
	}
	if _, err := session.Run([]string{
		"target",
	}, &Registry{}); err != nil {
		t.Fatal(err)
	}
	if runs.Load() != 2 {
		t.Fatalf("stable retry was not cached; runner calls = %d", runs.Load())
	}
}

func TestSessionRejectsContinuallyChangingGeneration(t *testing.T) {
	var fingerprints atomic.Int32
	var runs atomic.Int32
	session := newSession(
		SessionOptions{
			MaxEntries: 2,
			MaxBytes:   1 << 20,
		},
		func([]string, *Registry) (analysisCacheKey, error) {
			return sha256.Sum256([]byte(fmt.Sprint(fingerprints.Add(1)))),
				nil
		},
		func([]string, *Registry) ([]diagnostic.Diagnostic, error) {
			runs.Add(1)
			return nil,
				nil
		},
	)
	if _, err := session.Run([]string{
		"target",
	}, &Registry{}); err == nil || !strings.Contains(err.Error(), "inputs changed") {
		t.Fatalf("continually changing inputs returned error %v", err)
	}
	if runs.Load() != maxGenerationAttempts {
		t.Fatalf("runner calls = %d, want %d", runs.Load(), maxGenerationAttempts)
	}
	if stats := session.Stats(); stats.Entries != 0 {
		t.Fatalf("unstable generation was cached: %#v", stats)
	}
}

func TestSessionInvalidationAndDeterministicEviction(t *testing.T) {
	var runs atomic.Int32
	session := newSession(
		SessionOptions{
			MaxEntries: 2,
			MaxBytes:   1 << 20,
		},
		labelFingerprint,
		func(paths []string, _ *Registry) ([]diagnostic.Diagnostic, error) {
			runs.Add(1)
			return []diagnostic.Diagnostic{
					{
						Code: paths[0],
					},
				},
				nil
		},
	)
	registry := &Registry{}
	for _, target := range []string{
		"a",
		"b",
		"c",
		"b",
		"a",
	} {
		if _, err := session.Run([]string{
			target,
		}, registry); err != nil {
			t.Fatal(err)
		}
	}
	if runs.Load() != 4 {
		t.Fatalf("runner calls = %d, want 4", runs.Load())
	}
	stats := session.Stats()
	if stats.Hits != 1 || stats.Misses != 4 || stats.Evictions != 2 || stats.Entries != 2 {
		t.Fatalf("unexpected eviction stats: %#v", stats)
	}
	session.Invalidate()
	if stats := session.Stats(); stats.Entries != 0 || stats.Bytes != 0 {
		t.Fatalf("invalidate retained results: %#v", stats)
	}
	if _, err := session.Run([]string{
		"b",
	}, registry); err != nil {
		t.Fatal(err)
	}
	if runs.Load() != 5 {
		t.Fatalf("invalidated result was reused; runner calls = %d", runs.Load())
	}
}

func TestSessionDoesNotRetainOversizedResult(t *testing.T) {
	var runs atomic.Int32
	session := newSession(
		SessionOptions{
			MaxEntries: 2,
			MaxBytes:   1,
		},
		labelFingerprint,
		func([]string, *Registry) ([]diagnostic.Diagnostic, error) {
			runs.Add(1)
			return []diagnostic.Diagnostic{
					{
						Code:    "check",
						Message: "large",
					},
				},
				nil
		},
	)
	for range 2 {
		if _, err := session.Run([]string{
			"target",
		}, &Registry{}); err != nil {
			t.Fatal(err)
		}
	}
	if runs.Load() != 2 {
		t.Fatalf("oversized result was reused; runner calls = %d", runs.Load())
	}
	if stats := session.Stats(); stats.Entries != 0 || stats.Bytes != 0 {
		t.Fatalf("oversized result was retained: %#v", stats)
	} else if stats.Generation != 1 {
		t.Fatalf("unchanged oversized result published %d generations, want 1", stats.Generation)
	}
}

func TestAnalysisFingerprintTracksContentEnvironmentAndSettings(t *testing.T) {
	root := analysisModule(t, `package sample

import "time"

func check() { time.Sleep(1) }
`)
	filename := filepath.Join(root, "main.go")
	registry, err := NewRegistry(
		RegistryOptions{
			Only: []string{
				"suspicious-sleep",
			},
			Settings: map[string]config.CheckConfig{
				"suspicious-sleep": {
					Severity: "warning",
					Excludes: []string{
						"second.go",
						"first.go",
					},
				},
			},
			Root: root,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	first, err := analysisFingerprint([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	changed := `package sample

import "time"

func check() { time.Sleep(2) }
`
	if err := os.WriteFile(filename, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filename, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	contentChanged, err := analysisFingerprint([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if first == contentChanged {
		t.Fatal("same-size, same-mtime source mutation did not invalidate fingerprint")
	}

	reordered, err := NewRegistry(
		RegistryOptions{
			Only: []string{
				"suspicious-sleep",
			},
			Settings: map[string]config.CheckConfig{
				"suspicious-sleep": {
					Severity: "warning",
					Excludes: []string{
						"first.go",
						"second.go",
					},
				},
			},
			Root: root,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	reorderedKey, err := analysisFingerprint([]string{
		root,
	}, reordered)
	if err != nil {
		t.Fatal(err)
	}
	if reorderedKey != contentChanged {
		t.Fatal("order-independent excludes changed the fingerprint")
	}

	differentSeverity, err := NewRegistry(
		RegistryOptions{
			Only: []string{
				"suspicious-sleep",
			},
			Settings: map[string]config.CheckConfig{
				"suspicious-sleep": {
					Severity: "error",
					Excludes: []string{
						"first.go",
						"second.go",
					},
				},
			},
			Root: root,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	severityKey, err := analysisFingerprint([]string{
		root,
	}, differentSeverity)
	if err != nil {
		t.Fatal(err)
	}
	if severityKey == contentChanged {
		t.Fatal("effective severity did not invalidate the fingerprint")
	}

	t.Setenv("GOFLAGS", "-tags=strider_session_fingerprint")
	environmentKey, err := analysisFingerprint([]string{
		root,
	}, reordered)
	if err != nil {
		t.Fatal(err)
	}
	if environmentKey == reorderedKey {
		t.Fatal("GOFLAGS build tags did not invalidate the fingerprint")
	}
}

func TestSessionReusesAndInvalidatesRealTarget(t *testing.T) {
	root := analysisModule(t, `package sample

import "time"

func check() { time.Sleep(1) }
`)
	registry, err := newRegistry([]string{
		"suspicious-sleep",
	})
	if err != nil {
		t.Fatal(err)
	}
	session := NewSession(SessionOptions{
		MaxEntries: 2,
		MaxBytes:   1 << 20,
	})
	first, err := session.Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	second, err := session.Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(first) != fmt.Sprint(second) || len(first) != 1 {
		t.Fatalf("unexpected cached diagnostics: first %#v, second %#v", first, second)
	}
	if stats := session.Stats(); stats.Hits != 1 || stats.Misses != 1 {
		t.Fatalf("unchanged target was not reused: %#v", stats)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package sample\nfunc check() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	third, err := session.Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(third) != 0 {
		t.Fatalf("changed target reused stale diagnostics: %#v", third)
	}
	if stats := session.Stats(); stats.Misses != 2 {
		t.Fatalf("changed target did not miss: %#v", stats)
	}
}

func TestSessionRecursiveTargetDetectsNewSiblingPackage(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/sessiontopology\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(root, "existing")
	if err := os.Mkdir(existing, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(existing, "existing.go"), []byte("package existing\nfunc Value() int { return 1 }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		restoreSemanticWorkingDirectory(t, previousDirectory)
	})
	registry, err := newRegistry([]string{
		"suspicious-sleep",
	})
	if err != nil {
		t.Fatal(err)
	}
	session := NewSession(SessionOptions{
		MaxEntries: 4,
		MaxBytes:   1 << 20,
	})
	first, err := session.Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 0 {
		t.Fatalf("unexpected warm diagnostics: %#v", first)
	}

	sibling := filepath.Join(root, "sibling")
	if err := os.Mkdir(sibling, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sibling, "sibling.go"), []byte("package sibling\nimport \"time\"\nfunc Check() { time.Sleep(1) }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := session.Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].Code != "suspicious-sleep" {
		t.Fatalf("new sibling package reused stale diagnostics: %#v", second)
	}
	third, err := session.Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(second) != fmt.Sprint(third) {
		t.Fatalf("updated topology result was not reusable: second %#v, third %#v", second, third)
	}
	stats := session.Stats()
	if stats.Misses != 2 || stats.Hits != 1 {
		t.Fatalf("unexpected topology cache stats: %#v", stats)
	}
}

func TestSessionRecursiveTargetDetectsAddedNestedModuleBoundary(t *testing.T) {
	root, boundary, registry, session := nestedModuleBoundaryFixture(t, false)
	first, err := session.Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Code != "suspicious-sleep" {
		t.Fatalf("unexpected diagnostics before nested boundary: %#v", first)
	}
	if err := os.WriteFile(boundary, []byte("module example.com/nested\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := session.Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Fatalf("added nested boundary reused included-package diagnostics: %#v", second)
	}
	if stats := session.Stats(); stats.Misses != 2 || stats.Hits != 0 {
		t.Fatalf("added nested boundary did not invalidate the session: %#v", stats)
	}
}

func TestSessionRecursiveTargetDetectsDeletedNestedModuleBoundary(t *testing.T) {
	root, boundary, registry, session := nestedModuleBoundaryFixture(t, true)
	first, err := session.Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 0 {
		t.Fatalf("nested module unexpectedly contributed diagnostics: %#v", first)
	}
	if err := os.Remove(boundary); err != nil {
		t.Fatal(err)
	}
	second, err := session.Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].Code != "suspicious-sleep" {
		t.Fatalf("deleted nested boundary reused excluded-package diagnostics: %#v", second)
	}
	if stats := session.Stats(); stats.Misses != 2 || stats.Hits != 0 {
		t.Fatalf("deleted nested boundary did not invalidate the session: %#v", stats)
	}
}

func nestedModuleBoundaryFixture(t *testing.T, withBoundary bool) (string, string, *Registry, *Session) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/sessionboundary\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(root, "existing")
	if err := os.Mkdir(existing, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(existing, "existing.go"), []byte("package existing\nfunc Value() int { return 1 }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "nested.go"), []byte("package nested\nimport \"time\"\nfunc Check() { time.Sleep(1) }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	boundary := filepath.Join(nested, "go.mod")
	if withBoundary {
		if err := os.WriteFile(boundary, []byte("module example.com/nested\n\ngo 1.26\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		restoreSemanticWorkingDirectory(t, previousDirectory)
	})
	registry, err := newRegistry([]string{
		"suspicious-sleep",
	})
	if err != nil {
		t.Fatal(err)
	}
	return root, boundary, registry, NewSession(SessionOptions{
		MaxEntries: 4,
		MaxBytes:   1 << 20,
	})
}

func TestRecursiveTargetTopologySkipsIrrelevantTrees(t *testing.T) {
	root := t.TempDir()
	ignoredDirectories := []string{
		".hidden",
		"_generated",
		"vendor",
		"testdata",
	}
	for _, directory := range ignoredDirectories {
		path := filepath.Join(root, directory)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "ignored.go"), []byte("package ignored\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fingerprint := func() analysisCacheKey {
		writer := newFingerprintWriter()
		if err := addRecursiveTargetTopology(writer, root, nil); err != nil {
			t.Fatal(err)
		}
		return writer.sum()
	}
	first := fingerprint()
	for _, directory := range ignoredDirectories {
		if err := os.WriteFile(filepath.Join(root, directory, "new.go"), []byte("package changed\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if second := fingerprint(); second != first {
		t.Fatal("irrelevant recursive trees changed the topology fingerprint")
	}
	nodeModules := filepath.Join(root, "node_modules")
	if err := os.Mkdir(nodeModules, 0o700); err != nil {
		t.Fatal(err)
	}
	nodeModuleSource := filepath.Join(nodeModules, "module.go")
	if err := os.WriteFile(nodeModuleSource, []byte("package module\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nodeModulesKey := fingerprint()
	if nodeModulesKey == first {
		t.Fatal("node_modules package did not change the topology fingerprint")
	}
	if err := os.WriteFile(nodeModuleSource, []byte("package module\nfunc Changed() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nodeModulesContentKey := fingerprint()
	if nodeModulesContentKey == nodeModulesKey {
		t.Fatal("node_modules package content was not fingerprinted")
	}
	visible := filepath.Join(root, "visible")
	if err := os.Mkdir(visible, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(visible, "visible.go"), []byte("package visible\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if third := fingerprint(); third == nodeModulesContentKey {
		t.Fatal("visible package did not change the topology fingerprint")
	} else {
		if err := os.WriteFile(filepath.Join(visible, "visible.go"), []byte("package visible\nfunc Changed() {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if fourth := fingerprint(); fourth == third {
			t.Fatal("unloaded package content did not change the topology fingerprint")
		} else {
			boundary := filepath.Join(visible, "go.mod")
			if err := os.WriteFile(boundary, []byte("module example.com/visible-one\n\ngo 1.26\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			fifth := fingerprint()
			if fifth == fourth {
				t.Fatal("nested module boundary did not change the topology fingerprint")
			}
			if err := os.WriteFile(boundary, []byte("module example.com/visible-two\n\ngo 1.26\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if sixth := fingerprint(); sixth == fifth {
				t.Fatal("nested module boundary content was not fingerprinted")
			}
		}
	}
}

func TestDirectoryFingerprintTracksMembershipNotContents(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "required.go")
	if err := os.WriteFile(filename, []byte("package required\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint := func() analysisCacheKey {
		writer := newFingerprintWriter()
		if err := addDirectoryFingerprint(writer, root, false); err != nil {
			t.Fatal(err)
		}
		return writer.sum()
	}
	first := fingerprint()
	if err := os.WriteFile(filename, []byte("package required\nfunc Changed() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if second := fingerprint(); second != first {
		t.Fatal("file content changed a membership-only directory fingerprint")
	}
	if err := os.WriteFile(filepath.Join(root, "new.go"), []byte("package required\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if third := fingerprint(); third == first {
		t.Fatal("directory membership change did not change its fingerprint")
	}
}

func TestProbeConfigurationTracksVendorModulesMetadata(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "module")
	workRoot := filepath.Join(root, "workspace")
	if err := os.Mkdir(moduleRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(workRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	moduleFile := filepath.Join(moduleRoot, "go.mod")
	workFile := filepath.Join(workRoot, "go.work")
	if err := os.WriteFile(moduleFile, []byte("module example.com/vendorprobe\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workFile, []byte("go 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved, err := json.Marshal(map[string]string{
		"GOENV":  "",
		"GOMOD":  moduleFile,
		"GOWORK": workFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, base := range map[string]string{
		"gomod":  moduleRoot,
		"gowork": workRoot,
	} {
		t.Run(
			name,
			func(t *testing.T) {
				probe := &analysisProbe{
					scope: analysisProbeScope{
						cwd: moduleRoot,
					},
					resolvedEnvironment: resolved,
				}
				if err := probe.collectConfigurationFiles(); err != nil {
					t.Fatal(err)
				}
				probe.normalize()
				metadata := filepath.Join(base, "vendor", "modules.txt")
				found := false
				for _, filename := range probe.optionalFiles {
					if filename == metadata {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("vendor metadata %s is not a configuration input", metadata)
				}
				missing,
					err := probe.currentKey()
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(metadata), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(metadata, []byte("# first\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				created,
					err := probe.currentKey()
				if err != nil {
					t.Fatal(err)
				}
				if created == missing {
					t.Fatal("vendor metadata creation did not change the configuration key")
				}
				if err := os.WriteFile(metadata, []byte("# second\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				changed,
					err := probe.currentKey()
				if err != nil {
					t.Fatal(err)
				}
				if changed == created {
					t.Fatal("vendor metadata content did not change the configuration key")
				}
				if err := os.Remove(metadata); err != nil {
					t.Fatal(err)
				}
				deleted,
					err := probe.currentKey()
				if err != nil {
					t.Fatal(err)
				}
				if deleted != missing {
					t.Fatal("vendor metadata deletion did not restore the missing-file key")
				}
			},
		)
	}
}

func labelFingerprint(paths []string, _ *Registry) (analysisCacheKey, error) {
	if len(paths) == 0 {
		return analysisCacheKey{}, nil
	}
	return sha256.Sum256([]byte(paths[0])), nil
}

func BenchmarkSessionWholeTargetHit(benchmark *testing.B) {
	session := newSession(
		SessionOptions{
			MaxEntries: 2,
			MaxBytes:   1 << 20,
		},
		labelFingerprint,
		func([]string, *Registry) ([]diagnostic.Diagnostic, error) {
			return []diagnostic.Diagnostic{
					{
						Code:    "check",
						Message: "finding",
					},
				},
				nil
		},
	)
	registry := &Registry{}
	if _, err := session.Run([]string{
		"target",
	}, registry); err != nil {
		benchmark.Fatal(err)
	}
	benchmark.ResetTimer()
	for range benchmark.N {
		if _, err := session.Run([]string{
			"target",
		}, registry); err != nil {
			benchmark.Fatal(err)
		}
	}
}

func BenchmarkSessionWatchIteration(benchmark *testing.B) {
	root, changingFile, changingBase := benchmarkAnalysisModule(benchmark)
	previousDirectory, err := os.Getwd()
	if err != nil {
		benchmark.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		benchmark.Fatal(err)
	}
	benchmark.Cleanup(func() {
		restoreSemanticWorkingDirectory(benchmark, previousDirectory)
	})
	registry, err := newRegistry([]string{
		"possible-nil-dereference",
	})
	if err != nil {
		benchmark.Fatal(err)
	}
	benchmark.Run(
		"cold-run",
		func(benchmark *testing.B) {
			benchmark.ReportAllocs()
			for range benchmark.N {
				if _,
					err := Run([]string{
					root,
				}, registry); err != nil {
					benchmark.Fatal(err)
				}
			}
		},
	)
	benchmark.Run(
		"unchanged-session",
		func(benchmark *testing.B) {
			session := NewSession(SessionOptions{
				MaxEntries: 8,
				MaxBytes:   64 << 20,
			})
			if _,
				err := session.Run([]string{
				root,
			}, registry); err != nil {
				benchmark.Fatal(err)
			}
			benchmark.ReportAllocs()
			benchmark.ResetTimer()
			for range benchmark.N {
				if _,
					err := session.Run([]string{
					root,
				}, registry); err != nil {
					benchmark.Fatal(err)
				}
			}
		},
	)
	benchmark.Run(
		"single-file-change-session",
		func(benchmark *testing.B) {
			session := NewSession(SessionOptions{
				MaxEntries: 8,
				MaxBytes:   64 << 20,
			})
			if _,
				err := session.Run([]string{
				root,
			}, registry); err != nil {
				benchmark.Fatal(err)
			}
			benchmark.ReportAllocs()
			benchmark.ResetTimer()
			for index := range benchmark.N {
				benchmark.StopTimer()
				contents := fmt.Sprintf("// iteration %d\n%s", index, changingBase)
				if err := os.WriteFile(changingFile, []byte(contents), 0o600); err != nil {
					benchmark.Fatal(err)
				}
				benchmark.StartTimer()
				if _,
					err := session.Run([]string{
					root,
				}, registry); err != nil {
					benchmark.Fatal(err)
				}
			}
		},
	)
}

func benchmarkAnalysisModule(benchmark *testing.B) (string, string, string) {
	benchmark.Helper()
	root := benchmark.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/sessionbenchmark\n\ngo 1.26\n"), 0o600); err != nil {
		benchmark.Fatal(err)
	}
	for fileIndex := range 16 {
		var source strings.Builder
		source.WriteString("package benchmark\n\n")
		for functionIndex := range 64 {
			fmt.Fprintf(&source, "func check%d_%d(value *int) int { if value == nil {} ; return *value + %d }\n", fileIndex, functionIndex, functionIndex)
		}
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("checks_%02d.go", fileIndex)), []byte(source.String()), 0o600); err != nil {
			benchmark.Fatal(err)
		}
	}
	changingBase := "package benchmark\n\nfunc changing(value *int) int { if value == nil {} ; return *value }\n"
	changingFile := filepath.Join(root, "changing.go")
	if err := os.WriteFile(changingFile, []byte(changingBase), 0o600); err != nil {
		benchmark.Fatal(err)
	}
	return root, changingFile, changingBase
}
