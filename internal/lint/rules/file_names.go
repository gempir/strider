package rules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

func (a *analyzer) checkFilenameAndPackage() {
	base := filepath.Base(a.filename)
	validFile := regexp.MustCompile(`^[_A-Za-z0-9][_A-Za-z0-9-]*\.go$`)
	if a.on("filename-format") && !validFile.MatchString(base) {
		a.report(
			"filename-format",
			a.file.Name,
			"filename does not match the supported Go filename format",
		)
	}
	name := a.file.Name.Name
	validPackage := regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	if a.on("package-naming") && name != "main" && !validPackage.MatchString(name) {
		a.report(
			"package-naming",
			a.file.Name,
			"package name should be short, lower-case, and contain no separators",
		)
	}
	if a.on("package-directory-mismatch") && name != "main" && !pathContains(a.filename, "testdata") {
		directory := filepath.Base(filepath.Dir(a.filename))
		normalized := strings.ReplaceAll(strings.ReplaceAll(directory, "-", ""), "_", "")
		if normalized != "" && normalized != name && !strings.HasPrefix(directory, ".") {
			a.report(
				"package-directory-mismatch",
				a.file.Name,
				fmt.Sprintf("package %s does not match directory %s", name, directory),
			)
		}
	}
}

func pathContains(path, element string) bool {
	for _, part := range strings.Split(filepath.Clean(path), string(filepath.Separator)) {
		if part == element {
			return true
		}
	}
	return false
}
