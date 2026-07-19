// Package filewrite applies verified batches of source-file replacements.
package filewrite

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// ErrStale reports that a file no longer has the contents that were analyzed.
var ErrStale = errors.New("file changed since analysis")

var createTemporary = os.CreateTemp

var renameFile = os.Rename

// Change describes one complete file replacement. Before is the exact source
// that was analyzed and After is the source that should replace it.
type Change struct {
	Path   string
	Before []byte
	After  []byte
}

// Guard describes an analyzed file that must remain unchanged while a batch is
// applied. Guarded files are verified but never written. ExpectedTarget may
// hold the absolute, resolved target captured during analysis to detect a
// symlink that was retargeted without changing the guarded bytes.
type Guard struct {
	Path           string
	Before         []byte
	ExpectedTarget string
}

type plannedFile struct {
	paths          []string
	target         string
	expectedTarget string
	before         []byte
	after          []byte
	mode           os.FileMode
	write          bool
}

type stagedFile struct {
	temporary string
	target    string
	planned   plannedFile
}

// WriteBatch verifies, stages, and replaces a batch of files. Guards are
// verified with the changes but are never written. Every changed file is staged
// before the first replacement. Paths that contain symlinks are resolved to
// their existing targets so the links themselves remain intact.
//
// WriteBatch verifies the original contents both before staging and after every
// output has been staged. A concurrent change detected at either point returns
// an error wrapping ErrStale without replacing any target.
func WriteBatch(changes []Change, guards ...Guard) error {
	planned, err := planFiles(changes, guards)
	if err != nil || len(planned) == 0 {
		return err
	}
	for index := range planned {
		mode, validateErr := validateCurrent(planned[index], 0, false)
		if validateErr != nil {
			return validateErr
		}
		planned[index].mode = mode
	}

	staged := make([]stagedFile, 0, len(planned))
	for _, change := range planned {
		if !change.write {
			continue
		}
		file, stageErr := stage(change)
		if stageErr != nil {
			return errors.Join(stageErr, cleanup(staged))
		}
		staged = append(staged, file)
	}
	for _, change := range planned {
		if _, validateErr := validateCurrent(change, change.mode, change.write); validateErr != nil {
			return errors.Join(validateErr, cleanup(staged))
		}
	}
	for index, file := range staged {
		if _, validateErr := validateCurrent(file.planned, file.planned.mode, true); validateErr != nil {
			return errors.Join(validateErr, cleanup(staged[index:]))
		}
		if renameErr := renameFile(file.temporary, file.target); renameErr != nil {
			return errors.Join(renameErr, cleanup(staged[index:]))
		}
	}
	return nil
}

func planFiles(changes []Change, guards []Guard) ([]plannedFile, error) {
	raw := make([]plannedFile, 0, len(changes)+len(guards))
	add := func(path, expectedTarget string, before, after []byte, write bool) error {
		kind := "guard"
		if write {
			kind = "change"
		}
		if path == "" {
			return fmt.Errorf("filewrite: %s path is empty", kind)
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", path, err)
		}
		if expectedTarget != "" {
			if !filepath.IsAbs(expectedTarget) {
				return fmt.Errorf("filewrite: guard expected target %s is not absolute", expectedTarget)
			}
			expectedTarget = filepath.Clean(expectedTarget)
		}
		raw = append(
			raw,
			plannedFile{
				paths: []string{
					filepath.Clean(absolute),
				},
				expectedTarget: expectedTarget,
				before:         append([]byte(nil), before...),
				after:          append([]byte(nil), after...),
				write:          write,
			},
		)
		return nil
	}
	for _, change := range changes {
		if bytes.Equal(change.Before, change.After) {
			continue
		}
		if err := add(change.Path, "", change.Before, change.After, true); err != nil {
			return nil, err
		}
	}
	for _, guard := range guards {
		if err := add(guard.Path, guard.ExpectedTarget, guard.Before, nil, false); err != nil {
			return nil, err
		}
	}
	sort.Slice(raw, func(left, right int) bool {
		return raw[left].paths[0] < raw[right].paths[0]
	})
	infos := make([]os.FileInfo, len(raw))
	identities := make(map[[sha256.Size]byte][]int)
	for index := range raw {
		target, err := filepath.EvalSymlinks(raw[index].paths[0])
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", raw[index].paths[0], err)
		}
		raw[index].target = filepath.Clean(target)
		if raw[index].expectedTarget != "" && raw[index].expectedTarget != raw[index].target {
			return nil, staleError(raw[index].paths[0], "resolved target differs from analyzed target")
		}
		info, err := os.Stat(raw[index].target)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", raw[index].target, err)
		}
		digest := sha256.Sum256(raw[index].before)
		for _, previous := range identities[digest] {
			if raw[previous].target != raw[index].target && os.SameFile(infos[previous], info) {
				return nil, fmt.Errorf("filewrite: %s and %s identify the same filesystem file", raw[previous].paths[0], raw[index].paths[0])
			}
		}
		infos[index] = info
		identities[digest] = append(identities[digest], index)
	}
	sort.Slice(
		raw,
		func(left, right int) bool {
			if raw[left].target != raw[right].target {
				return raw[left].target < raw[right].target
			}
			if raw[left].paths[0] != raw[right].paths[0] {
				return raw[left].paths[0] < raw[right].paths[0]
			}
			return raw[left].write && !raw[right].write
		},
	)
	planned := make([]plannedFile, 0, len(raw))
	for start := 0; start < len(raw); {
		end := start + 1
		for end < len(raw) && raw[end].target == raw[start].target {
			end++
		}
		selected := start
		changesForTarget := 0
		expectedTarget := ""
		expectedPath := ""
		for index := start; index < end; index++ {
			if !bytes.Equal(raw[start].before, raw[index].before) {
				return nil, fmt.Errorf("filewrite: %s and %s have conflicting Before contents for %s", raw[start].paths[0], raw[index].paths[0], raw[start].target)
			}
			if raw[index].write {
				selected = index
				changesForTarget++
			}
			if raw[index].expectedTarget != "" {
				if expectedTarget != "" && expectedTarget != raw[index].expectedTarget {
					return nil, fmt.Errorf("filewrite: guards %s and %s have conflicting ExpectedTarget values", expectedPath, raw[index].paths[0])
				}
				expectedTarget = raw[index].expectedTarget
				expectedPath = raw[index].paths[0]
			}
		}
		if changesForTarget > 1 {
			return nil, fmt.Errorf("filewrite: multiple changes resolve to the same target %s", raw[start].target)
		}
		file := raw[selected]
		file.expectedTarget = expectedTarget
		file.paths = make([]string, 0, end-start)
		for index := start; index < end; index++ {
			path := raw[index].paths[0]
			if len(file.paths) == 0 || file.paths[len(file.paths)-1] != path {
				file.paths = append(file.paths, path)
			}
		}
		planned = append(planned, file)
		start = end
	}
	return planned, nil
}

func validateCurrent(change plannedFile, expectedMode os.FileMode, checkMode bool) (os.FileMode, error) {
	for _, path := range change.paths {
		target, err := filepath.EvalSymlinks(path)
		if err != nil {
			return 0, fmt.Errorf("resolve %s: %w", path, err)
		}
		if filepath.Clean(target) != change.target {
			return 0, staleError(path, "resolved target changed")
		}
	}
	info, err := os.Stat(change.target)
	if err != nil {
		return 0, fmt.Errorf("stat %s: %w", change.target, err)
	}
	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("filewrite: %s is not a regular file", change.target)
	}
	current, err := os.ReadFile(change.target)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", change.target, err)
	}
	if !bytes.Equal(current, change.before) {
		return 0, staleError(change.paths[0], "contents differ")
	}
	mode := info.Mode().Perm()
	if checkMode && mode != expectedMode {
		return 0, staleError(change.paths[0], "permissions changed")
	}
	return mode, nil
}

func staleError(path, reason string) error {
	return fmt.Errorf("%s: %w (%s)", path, ErrStale, reason)
}

func stage(change plannedFile) (stagedFile, error) {
	temporary, err := createTemporary(filepath.Dir(change.target), ".strider-*")
	if err != nil {
		return stagedFile{}, fmt.Errorf("stage %s: %w", change.target, err)
	}
	name := temporary.Name()
	fail := func(cause error, closed bool) (stagedFile, error) {
		if !closed {
			cause = errors.Join(cause, temporary.Close())
		}
		return stagedFile{}, errors.Join(cause, removeTemporary(name))
	}
	if err := temporary.Chmod(change.mode); err != nil {
		return fail(fmt.Errorf("stage %s: set permissions: %w", change.target, err), false)
	}
	written, err := temporary.Write(change.after)
	if err != nil {
		return fail(fmt.Errorf("stage %s: write: %w", change.target, err), false)
	}
	if written != len(change.after) {
		return fail(fmt.Errorf("stage %s: %w", change.target, io.ErrShortWrite), false)
	}
	if err := temporary.Close(); err != nil {
		return fail(fmt.Errorf("stage %s: close: %w", change.target, err), true)
	}
	return stagedFile{
		temporary: name,
		target:    change.target,
		planned:   change,
	}, nil
}

func cleanup(files []stagedFile) error {
	var err error
	for _, file := range files {
		err = errors.Join(err, removeTemporary(file.temporary))
	}
	return err
}

func removeTemporary(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
