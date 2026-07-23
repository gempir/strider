package filewrite

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteBatchRejectsStaleSameSizeContent(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "main.go")
	before := []byte("package one\n")
	stale := []byte("package two\n")
	if len(before) != len(stale) {
		t.Fatal("fixture contents must have equal lengths")
	}
	if err := os.WriteFile(path, stale, 0o640); err != nil {
		t.Fatal(err)
	}

	err := WriteBatch([]Change{
		{
			Path:   path,
			Before: before,
			After:  []byte("package fixed\n"),
		},
	})
	if !errors.Is(err, ErrStale) {
		t.Fatalf("WriteBatch error = %v, want ErrStale", err)
	}
	assertContents(t, path, stale)
}

func TestWriteBatchStaleGuardPreventsAllWrites(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	target := filepath.Join(directory, "a.go")
	guarded := filepath.Join(directory, "z.go")
	targetBefore := []byte("package target\n")
	guardBefore := []byte("package before\n")
	guardStale := []byte("package stale!\n")
	if len(guardBefore) != len(guardStale) {
		t.Fatal("fixture contents must have equal lengths")
	}
	if err := os.WriteFile(target, targetBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(guarded, guardStale, 0o600); err != nil {
		t.Fatal(err)
	}

	createCalls := 0
	operations := defaultFileOperations()
	operations.createTemporary = func(directory, pattern string) (*os.File, error) {
		createCalls++
		return os.CreateTemp(directory, pattern)
	}

	err := writeBatch(operations, []Change{
		{
			Path:   target,
			Before: targetBefore,
			After:  []byte("package fixed\n"),
		},
	}, Guard{
		Path:   guarded,
		Before: guardBefore,
	})
	if !errors.Is(err, ErrStale) {
		t.Fatalf("WriteBatch error = %v, want ErrStale", err)
	}
	if createCalls != 0 {
		t.Fatalf("staged %d file(s) before validating every guard", createCalls)
	}
	assertContents(t, target, targetBefore)
	assertContents(t, guarded, guardStale)
}

func TestWriteBatchRechecksGuardAfterStaging(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	target := filepath.Join(directory, "a.go")
	guarded := filepath.Join(directory, "z.go")
	targetBefore := []byte("package target\n")
	guardBefore := []byte("package before\n")
	guardStale := []byte("package stale!\n")
	if len(guardBefore) != len(guardStale) {
		t.Fatal("fixture contents must have equal lengths")
	}
	if err := os.WriteFile(target, targetBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(guarded, guardBefore, 0o600); err != nil {
		t.Fatal(err)
	}

	operations := defaultFileOperations()
	operations.createTemporary = func(directory, pattern string) (*os.File, error) {
		file, err := os.CreateTemp(directory, pattern)
		if err != nil {
			return nil, err
		}
		if writeErr := os.WriteFile(guarded, guardStale, 0o600); writeErr != nil {
			return nil, errors.Join(writeErr, file.Close(), os.Remove(file.Name()))
		}
		return file, nil
	}
	err := writeBatch(operations, []Change{
		{
			Path:   target,
			Before: targetBefore,
			After:  []byte("package fixed\n"),
		},
	}, Guard{
		Path:   guarded,
		Before: guardBefore,
	})
	if !errors.Is(err, ErrStale) {
		t.Fatalf("WriteBatch error = %v, want ErrStale", err)
	}
	assertContents(t, target, targetBefore)
	assertContents(t, guarded, guardStale)
	assertNoTemporaryFiles(t, directory)
}

func TestWriteBatchCoalescesMatchingGuardAndChange(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target.go")
	link := filepath.Join(directory, "link.go")
	before := []byte("package before\n")
	after := []byte("package after\n")
	if err := os.WriteFile(target, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(target), link); err != nil {
		t.Skipf("create symlink: %v", err)
	}

	if err := WriteBatch([]Change{
		{
			Path:   target,
			Before: before,
			After:  after,
		},
	}, Guard{
		Path:   link,
		Before: before,
	}); err != nil {
		t.Fatal(err)
	}
	assertContents(t, target, after)
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("guard alias mode = %v, want symlink", info.Mode())
	}
}

func TestWriteBatchRejectsConflictingGuardAndChange(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target.go")
	link := filepath.Join(directory, "link.go")
	before := []byte("package before\n")
	if err := os.WriteFile(target, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(target), link); err != nil {
		t.Skipf("create symlink: %v", err)
	}

	err := WriteBatch([]Change{
		{
			Path:   target,
			Before: before,
			After:  []byte("package after\n"),
		},
	}, Guard{
		Path:   link,
		Before: []byte("package other\n"),
	})
	if err == nil || !strings.Contains(err.Error(), "conflicting Before") {
		t.Fatalf("WriteBatch error = %v, want conflicting Before error", err)
	}
	assertContents(t, target, before)
}

func TestWriteBatchRejectsRetargetedGuardWithIdenticalContents(t *testing.T) {
	directory := t.TempDir()
	first := filepath.Join(directory, "first.go")
	second := filepath.Join(directory, "second.go")
	link := filepath.Join(directory, "guard.go")
	before := []byte("package same\n")
	for _, path := range []string{
		first,
		second,
	} {
		if err := os.WriteFile(path, before, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Base(first), link); err != nil {
		t.Skipf("create symlink: %v", err)
	}
	expectedTarget, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(second), link); err != nil {
		t.Fatal(err)
	}

	err = WriteBatch([]Change{
		{
			Path:   first,
			Before: before,
			After:  []byte("package changed\n"),
		},
	}, Guard{
		Path:           link,
		Before:         before,
		ExpectedTarget: expectedTarget,
	})
	if !errors.Is(err, ErrStale) {
		t.Fatalf("WriteBatch error = %v, want ErrStale", err)
	}
	assertContents(t, first, before)
	assertContents(t, second, before)
	assertNoTemporaryFiles(t, directory)
}

func TestWriteBatchPreservesPermissions(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "main.go")
	before := []byte("package before\n")
	after := []byte("package after\n")
	if err := os.WriteFile(path, before, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	beforeInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	beforeMode := beforeInfo.Mode().Perm()

	if err := WriteBatch([]Change{
		{
			Path:   path,
			Before: before,
			After:  after,
		},
	}); err != nil {
		t.Fatal(err)
	}
	assertContents(t, path, after)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != beforeMode {
		t.Fatalf("permissions = %v, want preserved mode %v", info.Mode().Perm(), beforeMode)
	}
}

func TestWriteBatchPreservesSymlink(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target.go")
	link := filepath.Join(directory, "link.go")
	before := []byte("package before\n")
	after := []byte("package after\n")
	if err := os.WriteFile(target, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(target), link); err != nil {
		t.Skipf("create symlink: %v", err)
	}

	if err := WriteBatch([]Change{
		{
			Path:   link,
			Before: before,
			After:  after,
		},
	}); err != nil {
		t.Fatal(err)
	}
	assertContents(t, target, after)
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("write replaced the symlink")
	}
	destination, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if destination != filepath.Base(target) {
		t.Fatalf("symlink target = %q, want %q", destination, filepath.Base(target))
	}
}

func TestWriteBatchStagesEveryFileBeforeReplacingTargets(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	first := filepath.Join(directory, "a.go")
	second := filepath.Join(directory, "z.go")
	before := []byte("package before\n")
	for _, path := range []string{
		first,
		second,
	} {
		if err := os.WriteFile(path, before, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	calls := 0
	operations := defaultFileOperations()
	operations.createTemporary = func(directory, pattern string) (*os.File, error) {
		calls++
		if calls == 2 {
			return nil, errors.New("injected staging failure")
		}
		return os.CreateTemp(directory, pattern)
	}
	err := writeBatch(operations, []Change{
		{
			Path:   second,
			Before: before,
			After:  []byte("package second\n"),
		},
		{
			Path:   first,
			Before: before,
			After:  []byte("package first\n"),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "injected staging failure") {
		t.Fatalf("WriteBatch error = %v, want injected failure", err)
	}
	if calls != 2 {
		t.Fatalf("CreateTemp calls = %d, want 2", calls)
	}
	assertContents(t, first, before)
	assertContents(t, second, before)
	assertNoTemporaryFiles(t, directory)
}

func TestWriteBatchRechecksContentsAfterStaging(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	first := filepath.Join(directory, "a.go")
	second := filepath.Join(directory, "z.go")
	before := []byte("package before\n")
	stale := []byte("package stale!\n")
	if len(before) != len(stale) {
		t.Fatal("fixture contents must have equal lengths")
	}
	for _, path := range []string{
		first,
		second,
	} {
		if err := os.WriteFile(path, before, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	calls := 0
	operations := defaultFileOperations()
	operations.createTemporary = func(directory, pattern string) (*os.File, error) {
		file, err := os.CreateTemp(directory, pattern)
		if err != nil {
			return nil, err
		}
		calls++
		if calls == 2 {
			if writeErr := os.WriteFile(first, stale, 0o600); writeErr != nil {
				return nil, errors.Join(writeErr, file.Close(), os.Remove(file.Name()))
			}
		}
		return file, nil
	}
	err := writeBatch(operations, []Change{
		{
			Path:   first,
			Before: before,
			After:  []byte("package first\n"),
		},
		{
			Path:   second,
			Before: before,
			After:  []byte("package second\n"),
		},
	})
	if !errors.Is(err, ErrStale) {
		t.Fatalf("WriteBatch error = %v, want ErrStale", err)
	}
	assertContents(t, first, stale)
	assertContents(t, second, before)
	assertNoTemporaryFiles(t, directory)
}

func TestWriteBatchRechecksEachTargetImmediatelyBeforeRename(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	first := filepath.Join(directory, "a.go")
	second := filepath.Join(directory, "z.go")
	before := []byte("package before\n")
	stale := []byte("package stale\n")
	for _, path := range []string{
		first,
		second,
	} {
		if err := os.WriteFile(path, before, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	renames := 0
	operations := defaultFileOperations()
	operations.rename = func(oldPath, newPath string) error {
		if err := os.Rename(oldPath, newPath); err != nil {
			return err
		}
		renames++
		if renames == 1 {
			return os.WriteFile(second, stale, 0o600)
		}
		return nil
	}
	err := writeBatch(operations, []Change{
		{
			Path:   first,
			Before: before,
			After:  []byte("package first\n"),
		},
		{
			Path:   second,
			Before: before,
			After:  []byte("package second\n"),
		},
	})
	if !errors.Is(err, ErrStale) {
		t.Fatalf("WriteBatch error = %v, want ErrStale", err)
	}
	if renames != 1 {
		t.Fatalf("renames = %d, want 1", renames)
	}
	assertContents(t, first, []byte("package first\n"))
	assertContents(t, second, stale)
	assertNoTemporaryFiles(t, directory)
}

func TestWriteBatchRejectsDuplicateResolvedTargets(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target.go")
	link := filepath.Join(directory, "link.go")
	before := []byte("package before\n")
	if err := os.WriteFile(target, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(target), link); err != nil {
		t.Skipf("create symlink: %v", err)
	}

	err := WriteBatch([]Change{
		{
			Path:   target,
			Before: before,
			After:  []byte("package target\n"),
		},
		{
			Path:   link,
			Before: before,
			After:  []byte("package link\n"),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "resolve to the same target") {
		t.Fatalf("WriteBatch error = %v, want duplicate target error", err)
	}
	assertContents(t, target, before)
}

func TestWriteBatchRejectsDuplicateFilesystemIdentity(t *testing.T) {
	directory := t.TempDir()
	first := filepath.Join(directory, "first.go")
	second := filepath.Join(directory, "second.go")
	before := []byte("package before\n")
	if err := os.WriteFile(first, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(first, second); err != nil {
		t.Skipf("create hard link: %v", err)
	}

	err := WriteBatch([]Change{
		{
			Path:   first,
			Before: before,
			After:  []byte("package first\n"),
		},
		{
			Path:   second,
			Before: before,
			After:  []byte("package second\n"),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "same filesystem file") {
		t.Fatalf("WriteBatch error = %v, want duplicate filesystem identity error", err)
	}
	assertContents(t, first, before)
	assertContents(t, second, before)
}

func TestWriteBatchUsesDeterministicTargetOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	firstDirectory := filepath.Join(root, "a")
	secondDirectory := filepath.Join(root, "z")
	for _, directory := range []string{
		firstDirectory,
		secondDirectory,
	} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	first := filepath.Join(firstDirectory, "main.go")
	second := filepath.Join(secondDirectory, "main.go")
	before := []byte("package before\n")
	for _, path := range []string{
		first,
		second,
	} {
		if err := os.WriteFile(path, before, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	order := []string{}
	operations := defaultFileOperations()
	operations.createTemporary = func(directory, pattern string) (*os.File, error) {
		order = append(order, directory)
		return os.CreateTemp(directory, pattern)
	}
	if err := writeBatch(
		operations,
		[]Change{
			{
				Path:   second,
				Before: before,
				After:  []byte("package second\n"),
			},
			{
				Path:   first,
				Before: before,
				After:  []byte("package first\n"),
			},
		},
	); err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(resolvedRoot, "a"),
		filepath.Join(resolvedRoot, "z"),
	}
	if strings.Join(order, "\n") != strings.Join(want, "\n") {
		t.Fatalf("staging order = %v, want %v", order, want)
	}
}

func TestWriteBatchIgnoresUnchangedChanges(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.go")
	contents := []byte("package same\n")
	if err := WriteBatch([]Change{
		{
			Path:   missing,
			Before: contents,
			After:  bytes.Clone(contents),
		},
	}); err != nil {
		t.Fatalf("unchanged change returned %v", err)
	}
}

func assertContents(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s contents = %q, want %q", path, got, want)
	}
}

func assertNoTemporaryFiles(t *testing.T, directory string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(directory, ".strider-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}
