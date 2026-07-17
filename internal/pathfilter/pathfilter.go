// Package pathfilter matches configuration paths and doublestar globs.
package pathfilter

import (
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

func Matches(root, filename string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	absolute, err := filepath.Abs(filename)
	if err != nil {
		return false
	}
	relative := absolute
	if root != "" {
		if candidate, relErr := filepath.Rel(root, absolute); relErr == nil {
			relative = candidate
		}
	}
	relative = filepath.ToSlash(filepath.Clean(relative))
	for _, pattern := range patterns {
		pattern = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(pattern)), "./")
		if strings.ContainsAny(pattern, "*?[") {
			matched, _ := doublestar.Match(pattern, relative)
			if matched {
				return true
			}
			continue
		}
		pattern = strings.TrimSuffix(pattern, "/")
		if relative == pattern || strings.HasPrefix(relative, pattern + "/") {
			return true
		}
	}
	return false
}
