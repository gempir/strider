package semantic

import (
	"fmt"
	"sort"

	"golang.org/x/tools/go/packages"
)

// ValidateOverlay parses and type-checks the packages containing paths with
// the supplied in-memory source files. Overlay keys must be absolute paths.
func ValidateOverlay(paths []string, overlay map[string][]byte) error {
	loadDirectory, patterns, _, err := loadInputs(".", paths)
	if err != nil {
		return err
	}
	loaded, err := packages.Load(&packages.Config{
		Dir:     loadDirectory,
		Mode:    loadMode,
		Tests:   true,
		Overlay: overlay,
	}, patterns...)
	if err != nil {
		return fmt.Errorf("validate fixes: %w", err)
	}
	if err := packageError(loaded); err != nil {
		return fmt.Errorf("validate fixes: %w", err)
	}
	if len(loaded) == 0 {
		return fmt.Errorf("validate fixes: no active package contains the changed source")
	}
	wanted := make(map[string]bool, len(paths))
	for _, path := range paths {
		canonical, err := canonicalPath(path)
		if err != nil {
			return fmt.Errorf("validate fixes: %w", err)
		}
		wanted[canonical] = true
	}
	removeCompiledPaths(wanted, loaded)
	if len(wanted) != 0 {
		inactive := make([]string, 0, len(wanted))
		for path := range wanted {
			inactive = append(inactive, path)
		}
		sort.Strings(inactive)
		return fmt.Errorf("validate fixes: %s is not part of an active package", inactive[0])
	}
	return nil
}

func removeCompiledPaths(wanted map[string]bool, loaded []*packages.Package) {
	for _, pkg := range loaded {
		for _, path := range pkg.CompiledGoFiles {
			canonical, err := canonicalPath(path)
			if err == nil {
				delete(wanted, canonical)
			}
		}
	}
}
