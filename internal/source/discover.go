// Package source provides deterministic Go source discovery.
//
//strider:ignore-file cognitive-complexity,modifies-parameter,no-package-var,single-case-switch
package source

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var workingDirectory = currentWorkingDirectory()

type Options struct {
	SkipGenerated bool
}

// DiscoverFrom resolves relative inputs against directory and returns
// deterministic absolute Go source paths.
func DiscoverFrom(directory string, paths []string, opts Options) ([]string, error) {
	if len(paths) == 0 {
		paths = []string{
			".",
		}
	}
	seen := make(map[string]struct{})
	for _, input := range paths {
		if err := discoverPath(seen, directory, input, opts); err != nil {
			return nil, err
		}
	}
	files := make([]string, 0, len(seen))
	for filename := range seen {
		files = append(files, filename)
	}
	sort.Strings(files)
	return files, nil
}

func discoverPath(seen map[string]struct{}, directory, input string, opts Options) error {
	path := cleanPattern(input)
	if !filepath.IsAbs(path) {
		path = filepath.Join(directory, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("discover %q: %w", input, err)
	}
	if !info.IsDir() {
		if filepath.Ext(path) != ".go" {
			return fmt.Errorf("%q is not a Go source file", input)
		}
		return addFile(seen, path, opts)
	}
	if err := filepath.WalkDir(path, walkSourceFiles(seen, path, opts)); err != nil {
		return fmt.Errorf("walk %q: %w", input, err)
	}
	return nil
}

func cleanPattern(input string) string {
	path := filepath.Clean(input)
	if !strings.HasSuffix(filepath.ToSlash(path), "/...") {
		return path
	}
	path = strings.TrimSuffix(path, string(filepath.Separator)+"...")
	if path == "" {
		return "."
	}
	return path
}

func walkSourceFiles(seen map[string]struct{}, root string, opts Options) fs.WalkDirFunc {
	return func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if filename != root && isSkippedDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".go" {
			return nil
		}
		return addFile(seen, filename, opts)
	}
}

func isSkippedDirectory(name string) bool {
	// Follow the Go convention for hidden and generated directory trees:
	// directories beginning with a dot or underscore are ignored unless the
	// user names one directly. This also prevents local tool caches from
	// silently becoming source input.
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return true
	}
	switch name {
	case "vendor":
		return true
	default:
		return false
	}
}

func addFile(seen map[string]struct{}, filename string, opts Options) error {
	abs, err := filepath.Abs(filename)
	if err != nil {
		return err
	}
	if opts.SkipGenerated {
		generated, err := IsGenerated(abs)
		if err != nil {
			return err
		}
		if generated {
			return nil
		}
	}
	seen[abs] = struct{}{}
	return nil
}

func IsGenerated(filename string) (bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, 4096))
	if err != nil {
		return false, err
	}
	return IsGeneratedSource(contents), nil
}

// IsGeneratedSource reports whether the first 4 KiB contains the canonical
// generated-code marker.
func IsGeneratedSource(contents []byte) bool {
	if len(contents) > 4096 {
		contents = contents[:4096]
	}
	scanner := bufio.NewScanner(bytes.NewReader(contents))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if bytes.HasPrefix(line, []byte("// Code generated ")) && bytes.HasSuffix(line, []byte(" DO NOT EDIT.")) {
			return true
		}
	}
	return false
}

func DisplayPath(filename string) string {
	return DiagnosticPath(workingDirectory, filename)
}

// DiagnosticPath returns the stable slash-separated spelling used by
// diagnostics. Files within root are root-relative; files outside it remain
// absolute. An empty root uses the process working directory captured once.
func DiagnosticPath(root, filename string) string {
	if root == "" {
		root = workingDirectory
	}
	absolute, err := filepath.Abs(filename)
	if err != nil {
		return filepath.ToSlash(filename)
	}
	absolute = filepath.Clean(absolute)
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = filepath.Clean(resolved)
	} else if resolvedDirectory, directoryErr := filepath.EvalSymlinks(filepath.Dir(absolute)); directoryErr == nil {
		absolute = filepath.Join(resolvedDirectory, filepath.Base(absolute))
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return filepath.ToSlash(absolute)
	}
	root = filepath.Clean(root)
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = filepath.Clean(resolved)
	}
	rel, err := filepath.Rel(root, absolute)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(absolute)
	}
	return filepath.ToSlash(rel)
}

// ResolveRoot returns one absolute, canonical diagnostic root. Callers keep
// the result for a complete check run instead of resolving the cwd per file.
func ResolveRoot(root string) string {
	if root == "" {
		root = currentWorkingDirectory()
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(absolute)
}

func currentWorkingDirectory() string {
	directory, err := os.Getwd()
	if err != nil {
		return "."
	}
	return directory
}
