//strider:ignore-file cognitive-complexity,cyclomatic-complexity
package checks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/resultcache"
	"github.com/gempir/strider/internal/telemetry"
	"github.com/gempir/strider/internal/workspace"
)

func TestRunRejectsPreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, nil, nil, RunOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
}

func TestRunDoesNotConsumeWorkspace(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "main.go"), []byte("package sample\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shared, err := workspace.Open([]string{
		directory,
	}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer shared.Close()
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"no-init",
		},
		MinimumSeverity: diagnostic.SeverityNote,
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := Run(context.Background(), shared, registry, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Run(context.Background(), shared, registry, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated Run changed result:\nfirst:  %#v\nsecond: %#v", first, second)
	}
}

func TestRunWithWorkspaceCacheObservesDependencyChanges(t *testing.T) {
	directory := t.TempDir()
	dependencyDirectory := filepath.Join(directory, "dep")
	if err := os.Mkdir(dependencyDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.com/watch\n\ngo 1.24\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dependency := filepath.Join(dependencyDirectory, "dep.go")
	if err := os.WriteFile(dependency, []byte("package dep\ntype Value struct{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "target.go")
	if err := os.WriteFile(target, []byte("package watch\nimport \"example.com/watch/dep\"\nfunc Use(value dep.Value) {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"copy-lock-value",
		},
		Directory: directory,
	})
	if err != nil {
		t.Fatal(err)
	}
	cache := workspace.NewCache(workspace.CacheOptions{})
	t.Cleanup(cache.Close)
	run := func() Result {
		shared, openErr := cache.Open([]string{
			target,
		}, workspace.Options{
			Directory: directory,
		})
		if openErr != nil {
			t.Fatal(openErr)
		}
		defer shared.Close()
		result, runErr := Run(context.Background(), shared, registry, RunOptions{})
		if runErr != nil {
			t.Fatal(runErr)
		}
		return result
	}
	if result := run(); len(result.Diagnostics) != 0 {
		t.Fatalf("initial diagnostics = %#v", result.Diagnostics)
	}
	if err := os.WriteFile(dependency, []byte("package dep\nimport \"sync\"\ntype Value struct { Mutex sync.Mutex }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed := run()
	if len(changed.Diagnostics) == 0 || changed.Diagnostics[0].Code != "copy-lock-value" {
		t.Fatalf("dependency-change diagnostics = %#v", changed.Diagnostics)
	}
	stats := cache.Stats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Fatalf("workspace reuse stats = %#v", stats)
	}
}

func TestConcreteWorkersCancelOnFirstErrorAndJoin(t *testing.T) {
	previous := runtime.GOMAXPROCS(4)
	t.Cleanup(func() {
		runtime.GOMAXPROCS(previous)
	})
	directory := t.TempDir()
	for index := range 32 {
		filename := filepath.Join(directory, fmt.Sprintf("file_%02d.go", index))
		if err := os.WriteFile(filename, []byte("package sample\nfunc init() {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	shared, err := workspace.Open([]string{
		directory,
	}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"no-init",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("worker failed")
	var started atomic.Int32
	var finished atomic.Int32
	var failed atomic.Bool
	runner := func(ctx context.Context, file *workspace.File, _ *Registry, _ *formatter.Formatter, _ formatter.Options, _ bool) fileResult {
		started.Add(1)
		defer finished.Add(1)
		if failed.CompareAndSwap(false, true) {
			return fileResult{
				filename: file.Path(),
				err:      sentinel,
			}
		}
		<-ctx.Done()
		return fileResult{
			filename: file.Path(),
			err:      ctx.Err(),
		}
	}
	_, err = runConcreteChecksWith(context.Background(), shared.Files(), registry, formatter.DefaultOptions(), false, runner)
	if !errors.Is(err, sentinel) {
		t.Fatalf("worker result = %v, want sentinel", err)
	}
	if got, want := finished.Load(), started.Load(); got != want {
		t.Fatalf("finished %d of %d started workers", got, want)
	}
	if started.Load() >= int32(len(shared.Files())) {
		t.Fatalf("first error did not stop admission: started %d tasks", started.Load())
	}
}

func TestConcreteWorkersJoinAfterMidRunCancellation(t *testing.T) {
	previous := runtime.GOMAXPROCS(4)
	t.Cleanup(func() {
		runtime.GOMAXPROCS(previous)
	})
	directory := t.TempDir()
	for index := range 16 {
		filename := filepath.Join(directory, fmt.Sprintf("file_%02d.go", index))
		if err := os.WriteFile(filename, []byte("package sample\nfunc init() {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	shared, err := workspace.Open([]string{
		directory,
	}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"no-init",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var started atomic.Int32
	var finished atomic.Int32
	startedSignal := make(chan struct{})
	var signalOnce sync.Once
	runner := func(ctx context.Context, file *workspace.File, _ *Registry, _ *formatter.Formatter, _ formatter.Options, _ bool) fileResult {
		started.Add(1)
		defer finished.Add(1)
		signalOnce.Do(func() {
			close(startedSignal)
		})
		<-ctx.Done()
		return fileResult{
			filename: file.Path(),
			err:      ctx.Err(),
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, runErr := runConcreteChecksWith(ctx, shared.Files(), registry, formatter.DefaultOptions(), false, runner)
		done <- runErr
	}()
	<-startedSignal
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("worker result = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled worker run did not complete")
	}
	if got, want := finished.Load(), started.Load(); got != want {
		t.Fatalf("finished %d of %d started workers", got, want)
	}
}

func TestRunIsDeterministicAcrossWorkersAndPackages(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/determinism\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, packageName := range []string{
		"alpha",
		"beta",
	} {
		directory := filepath.Join(root, packageName)
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		source := []byte("package " + packageName + "\nimport \"regexp\"\nfunc init(){regexp.MustCompile(\"[\")}\n")
		if err := os.WriteFile(filepath.Join(directory, packageName+".go"), source, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"format",
			"invalid-regexp",
			"no-init",
		},
		Root: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	previous := runtime.GOMAXPROCS(1)
	t.Cleanup(func() {
		runtime.GOMAXPROCS(previous)
	})
	var expected []diagnostic.Diagnostic
	for _, workers := range []int{
		1,
		4,
		2,
		8,
	} {
		runtime.GOMAXPROCS(workers)
		shared, err := workspace.Open([]string{
			root,
		}, workspace.Options{})
		if err != nil {
			t.Fatal(err)
		}
		result, err := Run(context.Background(), shared, registry, RunOptions{
			Formatter: formatter.DefaultOptions(),
			Root:      root,
		})
		if err != nil {
			t.Fatal(err)
		}
		if expected == nil {
			expected = result.Diagnostics
			continue
		}
		if !reflect.DeepEqual(result.Diagnostics, expected) {
			t.Fatalf("GOMAXPROCS=%d diagnostics changed:\nfirst: %#v\nnext:  %#v", workers, expected, result.Diagnostics)
		}
	}
	if len(expected) < 6 {
		t.Fatalf("determinism fixture produced only %d diagnostics: %#v", len(expected), expected)
	}
}

func TestRunSharesCSTBetweenFormattingAndSyntaxChecks(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "sample.go")
	original := []byte("package sample\nfunc init(){println(\"x\")}\n")
	if err := os.WriteFile(filename, original, 0o600); err != nil {
		t.Fatal(err)
	}
	shared, err := workspace.Open([]string{
		filename,
	}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"format",
			"no-init",
		},
		Root: directory,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), shared, registry, RunOptions{
		Formatter:         formatter.DefaultOptions(),
		CollectCandidates: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(result.Diagnostics), 2; got != want {
		t.Fatalf("diagnostic count = %d, want %d: %#v", got, want, result.Diagnostics)
	}
	if result.Diagnostics[0].Code != "format" || result.Diagnostics[1].Code != "no-init" {
		t.Fatalf("diagnostic codes = %q, %q", result.Diagnostics[0].Code, result.Diagnostics[1].Code)
	}
	assertDiagnosticContract(t, map[string][]byte{
		"sample.go": original,
	}, result.Diagnostics)
	candidate, ok := result.Candidates[filename]
	if !ok || !candidate.Changed {
		t.Fatal("missing changed formatting candidate")
	}
	contents, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contents, original) {
		t.Fatal("read-only check modified source")
	}
}

func TestRunExcludesFilterDiagnosticsAndCandidates(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "excluded.go")
	if err := os.WriteFile(filename, []byte("package sample\nfunc init( ){return}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shared, err := workspace.Open([]string{
		directory,
	}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"format",
			"no-init",
		},
		Root: directory,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(
		context.Background(),
		shared,
		registry,
		RunOptions{
			Formatter: formatter.DefaultOptions(),
			Root:      directory,
			Excludes: []string{
				"excluded.go",
			},
			CollectCandidates: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(shared.Files()) != 1 {
		t.Fatalf("excluded source was removed from workspace: %#v", shared.Files())
	}
	if len(result.Diagnostics) != 0 || len(result.Candidates) != 0 {
		t.Fatalf("excluded result = %#v", result)
	}
}

func TestRunCSTOnlyDoesNotRequireGoPackage(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "standalone.go")
	if err := os.WriteFile(filename, []byte("package standalone\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shared, err := workspace.Open([]string{
		filename,
	}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"no-init",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), shared, registry, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(result.Diagnostics), 1; got != want {
		t.Fatalf("diagnostic count = %d, want %d", got, want)
	}
}

func TestRunFormatIgnoreFastPathDoesNotParse(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "ignored.go")
	if err := os.WriteFile(filename, []byte("//strider:format-ignore\nthis is intentionally not Go\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shared, err := workspace.Open([]string{
		filename,
	}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"format",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), shared, registry, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 || len(result.Candidates) != 0 {
		t.Fatalf("ignored result = %#v", result)
	}
}

func TestRunFileLocalCacheHitAvoidsCSTParse(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.go")
	if err := os.WriteFile(filename, []byte("package sample\nfunc init() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache, err := resultcache.Open(resultcache.Options{
		Directory: filepath.Join(directory, "cache"),
	})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(RegistryOptions{
		Only: []string{
			"no-init",
		},
		Root: directory,
	})
	if err != nil {
		t.Fatal(err)
	}
	run := func() Result {
		shared, err := workspace.Open([]string{
			filename,
		}, workspace.Options{})
		if err != nil {
			t.Fatal(err)
		}
		defer shared.Close()
		result, err := Run(context.Background(), shared, registry, RunOptions{
			SkipPackageLoading: true,
			Cache:              cache,
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := run()
	if err := cache.Flush(); err != nil {
		t.Fatal(err)
	}
	telemetryPath := filepath.Join(directory, "telemetry.json")
	t.Setenv(telemetry.EnvironmentVariable, telemetryPath)
	telemetry.ConfigureFromEnvironment("test cache hit")
	second := run()
	if err := telemetry.Flush(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Diagnostics, second.Diagnostics) {
		t.Fatalf("cache hit diagnostics differ:\nfirst:  %+v\nsecond: %+v", first.Diagnostics, second.Diagnostics)
	}
	contents, err := os.ReadFile(telemetryPath)
	if err != nil {
		t.Fatal(err)
	}
	var report telemetry.Report
	if err := json.Unmarshal(contents, &report); err != nil {
		t.Fatal(err)
	}
	for _, phase := range report.Phases {
		if phase.Name == "cst.parse" {
			t.Fatalf("cache hit parsed a CST: %+v", phase)
		}
	}
}
