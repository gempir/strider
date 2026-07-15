// Package source provides deterministic Go source discovery.
package source

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Options struct {
	SkipGenerated bool
}

func Discover(paths []string, opts Options) ([]string, error) {
	if len(paths) == 0 {
		paths = []string{"."}
	}

	seen := make(map[string]struct{})
	for _, input := range paths {
		path := filepath.Clean(input)
		if strings.HasSuffix(filepath.ToSlash(path), "/...") {
			path = strings.TrimSuffix(path, string(filepath.Separator)+"...")
			if path == "" {
				path = "."
			}
		}

		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("discover %q: %w", input, err)
		}
		if !info.IsDir() {
			if filepath.Ext(path) != ".go" {
				return nil, fmt.Errorf("%q is not a Go source file", input)
			}
			if err := addFile(seen, path, opts); err != nil {
				return nil, err
			}
			continue
		}

		err = filepath.WalkDir(path, func(filename string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if filename != path && isSkippedDirectory(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".go" {
				return nil
			}
			return addFile(seen, filename, opts)
		})
		if err != nil {
			return nil, fmt.Errorf("walk %q: %w", input, err)
		}
	}

	files := make([]string, 0, len(seen))
	for filename := range seen {
		files = append(files, filename)
	}
	sort.Strings(files)
	return files, nil
}

func isSkippedDirectory(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", "vendor":
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

	limited := io.LimitReader(file, 4096)
	scanner := bufio.NewScanner(limited)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if bytes.HasPrefix(line, []byte("// Code generated ")) && bytes.HasSuffix(line, []byte(" DO NOT EDIT.")) {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return false, nil
}

func DisplayPath(filename string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return filepath.ToSlash(filename)
	}
	rel, err := filepath.Rel(cwd, filename)
	if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(filename)
	}
	return filepath.ToSlash(rel)
}
