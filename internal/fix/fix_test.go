package fix

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/filewrite"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/workspace"
)

func TestPlanComposesGranularEditsBeforeFormatting(t *testing.T) {
	filename, before, snapshot := fixFixture(t, "package sample\nfunc ready(value bool)bool{return !!value}\n")
	candidate, err := formatter.Format(filename, before)
	if err != nil || !candidate.Changed || !bytes.Contains(candidate.Source, []byte("!!value")) {
		t.Fatalf("formatter candidate = %#v, error = %v", candidate, err)
	}
	start := bytes.Index(before, []byte("!!value"))
	diagnostics := []diagnostic.Diagnostic{
		automaticDiagnostic(filename, "format", diagnostic.Safe, nil),
		automaticDiagnostic(filename, "double-negation", diagnostic.Safe, []diagnostic.TextEdit{
			{
				Start: start,
				End:   start + 1,
			},
			{
				Start: start + 1,
				End:   start + 2,
			},
		}),
	}

	result, err := Plan(snapshot, diagnostics, map[string]formatter.Result{
		filename: candidate,
	}, Options{
		Mode: SafeOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 2 || len(result.Changes) != 1 || len(result.Skipped) != 0 {
		t.Fatalf("plan = %#v", result)
	}
	if bytes.Contains(result.Changes[0].After, []byte("!!value")) || !bytes.Contains(result.Changes[0].After, []byte("return value")) {
		t.Fatalf("composed source:\n%s", result.Changes[0].After)
	}
}

func TestPlanSafeOnlyAndIncludeUnsafe(t *testing.T) {
	filename, before, snapshot := fixFixture(t, "package sample\n\nvar safeName = 1\nvar unsafeName = 2\n")
	safeStart := bytes.Index(before, []byte("safeName"))
	unsafeStart := bytes.Index(before, []byte("unsafeName"))
	diagnostics := []diagnostic.Diagnostic{
		automaticDiagnostic(filename, "safe", diagnostic.Safe, []diagnostic.TextEdit{
			{
				Start:   safeStart,
				End:     safeStart + len("safeName"),
				NewText: "fixedSafe",
			},
		}),
		automaticDiagnostic(
			filename,
			"unsafe",
			diagnostic.Unsafe,
			[]diagnostic.TextEdit{
				{
					Start:   unsafeStart,
					End:     unsafeStart + len("unsafeName"),
					NewText: "fixedUnsafe",
				},
			},
		),
	}

	safe, err := Plan(snapshot, diagnostics, nil, Options{
		Mode: SafeOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	if safe.Applied != 1 || len(safe.Changes) != 1 || !bytes.Contains(safe.Changes[0].After, []byte("fixedSafe")) || bytes.Contains(
		safe.Changes[0].After,
		[]byte("fixedUnsafe"),
	) {
		t.Fatalf("safe plan = %#v, source:\n%s", safe, safe.Changes[0].After)
	}

	all, err := Plan(snapshot, diagnostics, nil, Options{
		Mode: IncludeUnsafe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if all.Applied != 2 || len(all.Changes) != 1 || !bytes.Contains(all.Changes[0].After, []byte("fixedSafe")) || !bytes.Contains(all.Changes[0].After, []byte("fixedUnsafe")) {
		t.Fatalf("unsafe plan = %#v, source:\n%s", all, all.Changes[0].After)
	}
}

func TestPlanSkipsEveryNonIdenticalOverlappingFix(t *testing.T) {
	filename, before, snapshot := fixFixture(t, "package sample\n\nvar value = 1\n")
	start := bytes.Index(before, []byte("value"))
	diagnostics := []diagnostic.Diagnostic{
		automaticDiagnostic(filename, "first", diagnostic.Unsafe, []diagnostic.TextEdit{
			{
				Start:   start,
				End:     start + 3,
				NewText: "one",
			},
		}),
		automaticDiagnostic(filename, "second", diagnostic.Unsafe, []diagnostic.TextEdit{
			{
				Start:   start + 2,
				End:     start + 5,
				NewText: "two",
			},
		}),
	}

	result, err := Plan(snapshot, diagnostics, nil, Options{
		Mode: IncludeUnsafe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 0 || len(result.Changes) != 0 || len(result.Skipped) != 2 {
		t.Fatalf("plan = %#v", result)
	}
}

func TestPlanCoalescesOnlyIdenticalAtomicFixes(t *testing.T) {
	filename, before, snapshot := fixFixture(t, "package sample\n\nvar value = 1\n")
	start := bytes.Index(before, []byte("value"))
	edits := []diagnostic.TextEdit{
		{
			Start:   start,
			End:     start + len("value"),
			NewText: "other",
		},
	}
	diagnostics := []diagnostic.Diagnostic{
		automaticDiagnostic(filename, "first", diagnostic.Unsafe, edits),
		automaticDiagnostic(filename, "second", diagnostic.Unsafe, edits),
	}

	result, err := Plan(snapshot, diagnostics, nil, Options{
		Mode: IncludeUnsafe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 2 || len(result.Changes) != 1 || len(result.Skipped) != 0 || bytes.Count(result.Changes[0].After, []byte("other")) != 1 {
		t.Fatalf("plan = %#v, source:\n%s", result, result.Changes[0].After)
	}
}

func TestPlanKeepsPartiallySharedFixesAtomic(t *testing.T) {
	filename, before, snapshot := fixFixture(t, "package sample\n\nvar abc = 1\n")
	start := bytes.Index(before, []byte("abc"))
	shared := diagnostic.TextEdit{
		Start:   start + 1,
		End:     start + 2,
		OldText: "b",
	}
	diagnostics := []diagnostic.Diagnostic{
		automaticDiagnostic(filename, "first", diagnostic.Unsafe, []diagnostic.TextEdit{
			{
				Start:   start,
				End:     start + 1,
				OldText: "a",
			},
			shared,
		}),
		automaticDiagnostic(filename, "second", diagnostic.Unsafe, []diagnostic.TextEdit{
			shared,
			{
				Start:   start + 2,
				End:     start + 3,
				OldText: "c",
			},
		}),
	}

	result, err := Plan(snapshot, diagnostics, nil, Options{
		Mode: IncludeUnsafe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 0 || len(result.Changes) != 0 || len(result.Skipped) != 2 {
		t.Fatalf("partially shared plan = %#v", result)
	}
}

func TestPlanUsesByteOffsetsWithUTF8AndCRLF(t *testing.T) {
	filename, before, snapshot := fixFixture(t, "package sample\r\n\r\nvar label = \"λ\"\r\nvar value = 1\r\n")
	start := bytes.Index(before, []byte("value"))
	item := automaticDiagnostic(filename, "rename", diagnostic.Unsafe, []diagnostic.TextEdit{
		{
			Start:   start,
			End:     start + len("value"),
			NewText: "count",
		},
	})

	result, err := Plan(snapshot, []diagnostic.Diagnostic{
		item,
	}, nil, Options{
		Mode: IncludeUnsafe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Changes) != 1 || !bytes.Contains(result.Changes[0].After, []byte("\"λ\"")) || !bytes.Contains(result.Changes[0].After, []byte("count")) {
		t.Fatalf("plan = %#v", result)
	}
}

func TestPlanRejectsInvalidRangesAndAutomaticAlternatives(t *testing.T) {
	filename, before, snapshot := fixFixture(t, "package sample\n")
	invalid := automaticDiagnostic(filename, "invalid", diagnostic.Unsafe, []diagnostic.TextEdit{
		{
			Start: 0,
			End:   len(before) + 1,
		},
	})
	if _, err := Plan(snapshot, []diagnostic.Diagnostic{
		invalid,
	}, nil, Options{
		Mode: IncludeUnsafe,
	}); err == nil || !strings.Contains(err.Error(), "invalid edit range") {
		t.Fatalf("invalid range error = %v", err)
	}

	alternatives := invalid
	alternatives.Fixes = append(alternatives.Fixes, alternatives.Fixes[0])
	if _, err := Plan(snapshot, []diagnostic.Diagnostic{
		alternatives,
	}, nil, Options{
		Mode: IncludeUnsafe,
	}); err == nil || !strings.Contains(err.Error(), "more than one automatic fix") {
		t.Fatalf("automatic alternatives error = %v", err)
	}

	staleEdit := automaticDiagnostic(filename, "stale-edit", diagnostic.Unsafe, []diagnostic.TextEdit{
		{
			Start:   0,
			End:     len("package"),
			OldText: "missing",
		},
	})
	if _, err := Plan(snapshot, []diagnostic.Diagnostic{
		staleEdit,
	}, nil, Options{
		Mode: IncludeUnsafe,
	}); err == nil || !strings.Contains(err.Error(), "did not contain expected source") {
		t.Fatalf("expected source error = %v", err)
	}
}

func TestPlanRejectsSafeFixThatDoesNotTypeCheck(t *testing.T) {
	filename, before, snapshot := fixFixture(t, "package sample\n\nvar value = 1\n")
	start := bytes.Index(before, []byte("1"))
	item := automaticDiagnostic(filename, "break-types", diagnostic.Safe, []diagnostic.TextEdit{
		{
			Start:   start,
			End:     start + 1,
			NewText: "missing",
		},
	})
	if _, err := Plan(snapshot, []diagnostic.Diagnostic{
		item,
	}, nil, Options{
		Mode: SafeOnly,
	}); err == nil || !strings.Contains(err.Error(), "undefined: missing") {
		t.Fatalf("type-check error = %v", err)
	}
}

func TestApplyRejectsStaleSource(t *testing.T) {
	filename, before, snapshot := fixFixture(t, "package sample\n\nvar value = 1\n")
	start := bytes.Index(before, []byte("value"))
	item := automaticDiagnostic(filename, "rename", diagnostic.Unsafe, []diagnostic.TextEdit{
		{
			Start:   start,
			End:     start + len("value"),
			NewText: "other",
		},
	})
	result, err := Plan(snapshot, []diagnostic.Diagnostic{
		item,
	}, nil, Options{
		Mode: IncludeUnsafe,
	})
	if err != nil {
		t.Fatal(err)
	}
	stale := bytes.Replace(before, []byte("value"), []byte("stale"), 1)
	if err := os.WriteFile(filename, stale, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Apply(result); !errors.Is(err, filewrite.ErrStale) {
		t.Fatalf("Apply error = %v, want ErrStale", err)
	}
	contents, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contents, stale) {
		t.Fatalf("stale source changed to %q", contents)
	}
}

func TestApplyGuardsUnchangedAnalyzedFiles(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.com/guardfixture\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(directory, "first.go")
	second := filepath.Join(directory, "second.go")
	firstBefore := []byte("package sample\n\nvar firstValue = 1\n")
	secondBefore := []byte("package sample\n\nvar secondValue = 2\n")
	if err := os.WriteFile(first, firstBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, secondBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	shared, err := workspace.Open([]string{
		directory,
	}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := Capture(shared)
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(firstBefore, []byte("firstValue"))
	item := automaticDiagnostic(first, "rename", diagnostic.Unsafe, []diagnostic.TextEdit{
		{
			Start:   start,
			End:     start + len("firstValue"),
			NewText: "otherValue",
		},
	})
	result, err := Plan(snapshot, []diagnostic.Diagnostic{
		item,
	}, nil, Options{
		Mode: IncludeUnsafe,
	})
	if err != nil {
		t.Fatal(err)
	}
	secondStale := bytes.Replace(secondBefore, []byte("secondValue"), []byte("staleValue!"), 1)
	if len(secondStale) != len(secondBefore) {
		t.Fatal("stale fixture must retain its byte length")
	}
	if err := os.WriteFile(second, secondStale, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Apply(result); !errors.Is(err, filewrite.ErrStale) {
		t.Fatalf("Apply error = %v, want ErrStale", err)
	}
	contents, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contents, firstBefore) {
		t.Fatalf("planned source changed despite stale guard:\n%s", contents)
	}
}

func TestApplyRejectsRetargetedSourceSymlink(t *testing.T) {
	directory := t.TempDir()
	firstTarget := filepath.Join(directory, "first.go")
	secondTarget := filepath.Join(directory, "second.go")
	link := filepath.Join(directory, "linked.go")
	before := []byte("package sample\n\nvar value = 1\n")
	for _, path := range []string{
		firstTarget,
		secondTarget,
	} {
		if err := os.WriteFile(path, before, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Base(firstTarget), link); err != nil {
		t.Skipf("create symlink: %v", err)
	}
	shared, err := workspace.Open([]string{
		link,
	}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := Capture(shared)
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(before, []byte("value"))
	item := automaticDiagnostic(link, "rename", diagnostic.Unsafe, []diagnostic.TextEdit{
		{
			Start:   start,
			End:     start + len("value"),
			NewText: "other",
		},
	})
	result, err := Plan(snapshot, []diagnostic.Diagnostic{
		item,
	}, nil, Options{
		Mode: IncludeUnsafe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(secondTarget), link); err != nil {
		t.Fatal(err)
	}
	if err := Apply(result); !errors.Is(err, filewrite.ErrStale) {
		t.Fatalf("Apply error = %v, want ErrStale", err)
	}
	for _, path := range []string{
		firstTarget,
		secondTarget,
	} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(contents, before) {
			t.Fatalf("retargeted symlink changed %s:\n%s", path, contents)
		}
	}
}

func TestCaptureRejectsDuplicateFilesystemIdentity(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.com/hardlinkfixture\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(directory, "first.go")
	second := filepath.Join(directory, "second.go")
	before := []byte("package sample\n")
	if err := os.WriteFile(first, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(first, second); err != nil {
		t.Skipf("create hard link: %v", err)
	}
	shared, err := workspace.Open([]string{
		directory,
	}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Capture(shared); err == nil || !strings.Contains(err.Error(), "same source") {
		t.Fatalf("Capture error = %v, want duplicate filesystem identity error", err)
	}
}

func TestPlanMatchesFormatterExclusionsAgainstSourceSymlinkPath(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.com/symlinkexclude\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "target.go")
	link := filepath.Join(directory, "linked.go")
	before := []byte("package sample\n\nvar value=1\n")
	if err := os.WriteFile(target, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(target), link); err != nil {
		t.Skipf("create symlink: %v", err)
	}
	shared, err := workspace.Open([]string{
		link,
	}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := Capture(shared)
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(before, []byte("value"))
	item := automaticDiagnostic(link, "rename", diagnostic.Unsafe, []diagnostic.TextEdit{
		{
			Start:   start,
			End:     start + len("value"),
			NewText: "other",
		},
	})
	resolvedRoot, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Plan(snapshot, []diagnostic.Diagnostic{
		item,
	}, nil, Options{
		Mode: IncludeUnsafe,
		Root: resolvedRoot,
		FormatExcludes: []string{
			"linked.go",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Changes) != 1 || !bytes.Contains(result.Changes[0].After, []byte("var other=1")) {
		t.Fatalf("excluded symlink source was formatted:\n%s", result.Changes[0].After)
	}
}

func automaticDiagnostic(filename, code string, safety diagnostic.Safety, edits []diagnostic.TextEdit) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		File: filename,
		Code: code,
		Fixes: []diagnostic.Fix{
			{
				Message:   "apply test fix",
				Safety:    safety,
				Automatic: true,
				Edits:     edits,
			},
		},
	}
}

func fixFixture(t *testing.T, contents string) (string, []byte, Snapshot) {
	t.Helper()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.com/fixfixture\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(directory, "sample.go")
	before := []byte(contents)
	if err := os.WriteFile(filename, before, 0o600); err != nil {
		t.Fatal(err)
	}
	shared, err := workspace.Open([]string{
		filename,
	}, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := Capture(shared)
	if err != nil {
		t.Fatal(err)
	}
	return filename, before, snapshot
}
