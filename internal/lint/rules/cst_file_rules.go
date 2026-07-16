package rules

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gempir/strider/internal/cst"
)

func (a *cstAnalyzer) checkFilenameAndPackage() {
	nameToken := a.packageNameToken()
	if !nameToken.IsValid() {
		return
	}
	name := nameToken.Src()
	base := filepath.Base(a.filename)
	validFile := regexp.MustCompile(`^[_A-Za-z0-9][_A-Za-z0-9-]*\.go$`)
	if a.enabled["filename-format"] && !validFile.MatchString(base) {
		a.report("filename-format", nameToken, "filename does not match the supported Go filename format")
	}
	validPackage := regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	if a.enabled["package-naming"] && name != "main" && !validPackage.MatchString(name) {
		a.report(
			"package-naming",
			nameToken,
			"package name should be short, lower-case, and contain no separators",
		)
	}
	if a.enabled["package-directory-mismatch"] && name != "main" &&
		!pathContains(a.filename, "testdata") {
		directory := filepath.Base(filepath.Dir(a.filename))
		normalized := strings.ReplaceAll(strings.ReplaceAll(directory, "-", ""), "_", "")
		if normalized != "" && normalized != name && !strings.HasPrefix(directory, ".") {
			a.report(
				"package-directory-mismatch",
				nameToken,
				fmt.Sprintf("package %s does not match directory %s", name, directory),
			)
		}
	}
}

func (a *cstAnalyzer) checkConcreteImports() {
	seen := map[string]bool{}
	packageName := a.packageNameToken().Src()
	cst.Walk(a.tree.Root(), func(node cst.Node) bool {
		spec, ok := node.(*cst.ImportSpec)
		if !ok {
			return true
		}
		path, _ := strconv.Unquote(spec.ImportPath.Src())
		if seen[path] {
			a.report(
				"duplicated-imports",
				spec,
				fmt.Sprintf("package %s is imported more than once", path),
			)
		} else {
			seen[path] = true
		}
		alias := ""
		var aliasNode cst.Node
		switch {
		case spec.PERIOD.IsValid():
			alias, aliasNode = ".", spec.PERIOD
		case spec.PackageName.IsValid():
			alias, aliasNode = spec.PackageName.Src(), spec.PackageName
		}
		switch alias {
		case "":
		case ".":
			a.report("dot-imports", spec, "dot imports obscure where identifiers come from")
		case "_":
			if packageName != "main" && !strings.HasSuffix(a.filename, "_test.go") &&
				!concreteImportHasComment(a.tree, spec) {
				a.report("blank-imports", spec, "blank import should be justified by a comment")
			}
		default:
			if !regexp.MustCompile(`^[a-z][a-z0-9]*$`).MatchString(alias) {
				a.report(
					"import-alias-naming",
					aliasNode,
					"import alias should contain lower-case letters and digits",
				)
			}
			if alias == filepath.Base(path) {
				a.report(
					"redundant-import-alias",
					aliasNode,
					"import alias is identical to the package name",
				)
			}
		}
		importName := filepath.Base(path)
		if alias != "" && alias != "." && alias != "_" {
			importName = alias
		}
		if importName != "." && importName != "_" {
			a.importNames[importName] = true
		}
		return false
	})
}

func concreteImportHasComment(tree *cst.Tree, spec *cst.ImportSpec) bool {
	start, end := cst.Range(spec)
	startPosition := tree.Position(start)
	endPosition := tree.Position(end)
	source := tree.Source()
	for _, comment := range tree.Comments() {
		if comment.Line == endPosition.Line ||
			(comment.End <= start && strings.Count(string(source[comment.End:start]), "\n") <= 1 &&
				comment.Line+1 >= startPosition.Line) {
			return true
		}
	}
	return false
}

func (a *cstAnalyzer) checkLinesAndComments() {
	lines := bytes.Split(a.content, []byte("\n"))
	if a.enabled["bidirectional-control-character"] {
		for offset := 0; offset < len(a.content); {
			character, width := utf8.DecodeRune(a.content[offset:])
			if width == 0 {
				break
			}
			if bidirectionalControl(character) {
				a.reportRange(
					"bidirectional-control-character",
					offset,
					offset+width,
					fmt.Sprintf(
						"source contains invisible bidirectional control character U+%04X",
						character,
					),
				)
			}
			offset += width
		}
	}
	if a.enabled["line-length-limit"] {
		offset := 0
		for _, line := range lines {
			count := utf8.RuneCount(line)
			if count > 80 {
				a.reportRange(
					"line-length-limit",
					offset,
					offset+len(line),
					fmt.Sprintf("line has %d runes; maximum is 80", count),
				)
			}
			offset += len(line) + 1
		}
	}
	comments := a.tree.Comments()
	a.checkCommentSpacing(comments)
	a.checkPackageComment(comments)
	a.checkBuildTags(comments)
}

func (a *cstAnalyzer) checkCommentSpacing(comments []cst.Comment) {
	for _, comment := range comments {
		if a.enabled["comment-spacings"] && strings.HasPrefix(comment.Text, "//") &&
			len(comment.Text) > 2 && comment.Text[2] != ' ' && comment.Text[2] != '\t' &&
			!commentDirective(comment.Text[2:]) {
			a.reportRange(
				"comment-spacings",
				comment.Start,
				comment.End,
				"line comment should have a space after //",
			)
		}
		if !a.enabled["spaced-compiler-directive"] || comment.Column != 1 ||
			!strings.HasPrefix(comment.Text, "//") {
			continue
		}
		body := comment.Text[2:]
		trimmed := strings.TrimLeft(body, " \t")
		name := strings.TrimPrefix(trimmed, "go:")
		if len(trimmed) != len(body) && strings.HasPrefix(trimmed, "go:") && name != "" &&
			name[0] >= 'a' && name[0] <= 'z' {
			a.reportRange(
				"spaced-compiler-directive",
				comment.Start,
				comment.End,
				"compiler directive is ignored because whitespace follows //",
			)
		}
	}
}

func (a *cstAnalyzer) checkPackageComment(comments []cst.Comment) {
	if !a.enabled["package-comments"] || strings.HasSuffix(a.filename, "_test.go") {
		return
	}
	nameToken := a.packageNameToken()
	packageStart, _ := cst.Range(nameToken)
	for _, comment := range comments {
		if comment.End >= packageStart {
			break
		}
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(comment.Text, "//"), "/*"))
		text = strings.TrimSpace(strings.TrimSuffix(text, "*/"))
		if strings.HasPrefix(text, "Package "+nameToken.Src()) ||
			strings.HasPrefix(strings.TrimPrefix(text, "Package "), nameToken.Src()) {
			return
		}
	}
	a.report("package-comments", nameToken, "package should have a documentation comment")
}

func (a *cstAnalyzer) checkBuildTags(comments []cst.Comment) {
	if !a.enabled["redundant-build-tag"] {
		return
	}
	hasGoBuild := false
	plusBuild := []cst.Comment{}
	for _, comment := range comments {
		hasGoBuild = hasGoBuild || strings.HasPrefix(comment.Text, "//go:build")
		if strings.HasPrefix(comment.Text, "// +build") {
			plusBuild = append(plusBuild, comment)
		}
	}
	if hasGoBuild {
		for _, comment := range plusBuild {
			a.reportRange(
				"redundant-build-tag",
				comment.Start,
				comment.End,
				"legacy +build line is redundant with go:build",
			)
		}
	}
	for index, comment := range plusBuild {
		constraint := legacyBuildTerms(comment.Text)
		for earlier := range index {
			if slices.Equal(constraint, legacyBuildTerms(plusBuild[earlier].Text)) {
				a.reportRange(
					"redundant-build-tag",
					comment.Start,
					comment.End,
					"legacy build constraint duplicates an earlier constraint",
				)
				break
			}
		}
	}
}
