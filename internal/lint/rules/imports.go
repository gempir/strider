package rules

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func (a *analyzer) checkImports() {
	seen := map[string]*ast.ImportSpec{}
	for _, spec := range a.file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		if first := seen[path]; first != nil {
			a.report(
				"duplicated-imports",
				spec,
				fmt.Sprintf("package %s is imported more than once", path),
			)
		} else {
			seen[path] = spec
		}
		if spec.Name == nil {
			continue
		}
		alias := spec.Name.Name
		switch alias {
		case ".":
			a.report("dot-imports", spec, "dot imports obscure where identifiers come from")
		case "_":
			if a.file.Name.Name != "main" && !strings.HasSuffix(a.filename, "_test.go") &&
				spec.Doc == nil &&
				spec.Comment == nil {
				a.report("blank-imports", spec, "blank import should be justified by a comment")
			}
		default:
			if !regexp.MustCompile(`^[a-z][a-z0-9]*$`).MatchString(alias) {
				a.report(
					"import-alias-naming",
					spec.Name,
					"import alias should contain lower-case letters and digits",
				)
			}
			if alias == filepath.Base(path) {
				a.report(
					"redundant-import-alias",
					spec.Name,
					"import alias is identical to the package name",
				)
			}
		}
	}
}
