package filewrite

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolvedFile identifies one existing logical path and its current target.
// Same reports filesystem identity, including hard-link aliases.
type ResolvedFile struct {
	Path   string
	Target string
	info   os.FileInfo
}

// ResolveExisting resolves an existing path without changing it.
func ResolveExisting(path string) (ResolvedFile, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return ResolvedFile{}, fmt.Errorf("resolve %s: %w", path, err)
	}
	absolute = filepath.Clean(absolute)
	target, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return ResolvedFile{}, fmt.Errorf("resolve %s: %w", path, err)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return ResolvedFile{}, fmt.Errorf("resolve %s: %w", path, err)
	}
	target = filepath.Clean(target)
	info, err := os.Stat(target)
	if err != nil {
		return ResolvedFile{}, fmt.Errorf("stat %s: %w", target, err)
	}
	return ResolvedFile{
		Path:   absolute,
		Target: target,
		info:   info,
	}, nil
}

// Same reports whether two resolved paths identify the same filesystem file.
func (file ResolvedFile) Same(other ResolvedFile) bool {
	return file.info != nil && other.info != nil && os.SameFile(file.info, other.info)
}
