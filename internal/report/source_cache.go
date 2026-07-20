package report

import (
	"os"
	"path/filepath"
	"strings"
)

type sourceLineCache struct {
	root    string
	lines   map[string][]string
	missing map[string]bool
}

func newSourceLineCache(root string) *sourceLineCache {
	return &sourceLineCache{
		root:    root,
		lines:   make(map[string][]string),
		missing: make(map[string]bool),
	}
}

func (cache *sourceLineCache) read(filename string) []string {
	if cache.root != "" && !filepath.IsAbs(filename) {
		filename = filepath.Join(cache.root, filepath.FromSlash(filename))
	}
	if lines, ok := cache.lines[filename]; ok {
		return lines
	}
	if cache.missing[filename] {
		return nil
	}
	contents, err := os.ReadFile(filename)
	if err != nil {
		cache.missing[filename] = true
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(contents), "\r\n", "\n"), "\n")
	cache.lines[filename] = lines
	return lines
}
