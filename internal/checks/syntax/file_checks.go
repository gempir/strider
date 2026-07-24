//strider:ignore-file cognitive-complexity,no-package-var,single-case-switch,top-level-declaration-order
package syntax

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

var (
	validFilenamePattern = regexp.MustCompile(`^[_A-Za-z0-9][_A-Za-z0-9-]*\.go$`)
	validPackagePattern  = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
)

type filePackageFacts struct {
	nameToken   cst.Token
	name        string
	base        string
	packageName string
}

func (a *Pass) filePackageFacts() (filePackageFacts, bool) {
	nameToken := a.packageNameToken()
	if !nameToken.IsValid() {
		return filePackageFacts{}, false
	}
	name := nameToken.Src()
	base := filepath.Base(a.filename)
	packageName := name
	if strings.HasSuffix(base, "_test.go") && strings.HasSuffix(packageName, "_test") {
		packageName = strings.TrimSuffix(packageName, "_test")
	}
	return filePackageFacts{
		nameToken:   nameToken,
		name:        name,
		base:        base,
		packageName: packageName,
	}, true
}

func (a *Pass) checkFilenameFormat() {
	facts, ok := a.filePackageFacts()
	if ok && !validFilenamePattern.MatchString(facts.base) {
		a.Report(facts.nameToken, "filename does not match the supported Go filename format")
	}
}

func (a *Pass) checkPackageNaming() {
	facts, ok := a.filePackageFacts()
	if ok && facts.name != "main" && !validPackagePattern.MatchString(facts.packageName) {
		a.Report(facts.nameToken, "package name should be short, lower-case, and contain no separators")
	}
}

func (a *Pass) checkPackageDirectoryMismatch() {
	facts, ok := a.filePackageFacts()
	if ok && facts.packageName != "main" && !pathContains(a.filename, "testdata") {
		directory := filepath.Base(filepath.Dir(a.filename))
		normalized := strings.ReplaceAll(strings.ReplaceAll(directory, "-", ""), "_", "")
		if normalized != "" && normalized != facts.packageName && !strings.HasPrefix(directory, ".") {
			a.Report(facts.nameToken, fmt.Sprintf("package %s does not match directory %s", facts.name, directory))
		}
	}
}

type importFacts struct {
	spec      *cst.ImportSpec
	path      string
	alias     string
	aliasNode cst.Node
}

func parseImport(spec *cst.ImportSpec) (importFacts, bool) {
	path, err := strconv.Unquote(spec.ImportPath.Src())
	if err != nil {
		return importFacts{}, false
	}
	alias := ""
	var aliasNode cst.Node
	switch {
	case spec.PERIOD.IsValid():
		alias, aliasNode = ".", spec.PERIOD
	case spec.PackageName.IsValid():
		alias, aliasNode = spec.PackageName.Src(), spec.PackageName
	}
	return importFacts{
		spec:      spec,
		path:      path,
		alias:     alias,
		aliasNode: aliasNode,
	}, true
}

func (a *Pass) observeImport(spec *cst.ImportSpec) {
	facts, ok := parseImport(spec)
	if !ok {
		return
	}
	importName := filepath.Base(facts.path)
	if facts.alias != "" && facts.alias != "." && facts.alias != "_" {
		importName = facts.alias
	}
	if importName != "." && importName != "_" {
		a.imports().names[importName] = true
	}
}

func (a *Pass) checkImportsBlocklist(spec *cst.ImportSpec) {
	facts, ok := parseImport(spec)
	if !ok {
		return
	}
	for _, blocked := range a.stringsOption("blocked-imports") {
		if blocked == facts.path {
			a.Report(spec, fmt.Sprintf("import %s is blocked by configuration", facts.path))
			return
		}
	}
}

func (a *Pass) checkDuplicatedImports(spec *cst.ImportSpec) {
	facts, ok := parseImport(spec)
	if !ok {
		return
	}
	state := a.imports()
	if state.seen[facts.path] {
		a.Report(spec, fmt.Sprintf("package %s is imported more than once", facts.path))
	}
	state.seen[facts.path] = true
}

func (a *Pass) checkDotImports(spec *cst.ImportSpec) {
	facts, ok := parseImport(spec)
	if ok && facts.alias == "." {
		a.Report(spec, "dot imports obscure where identifiers come from")
	}
}

func (a *Pass) checkBlankImports(spec *cst.ImportSpec) {
	facts, ok := parseImport(spec)
	if ok && facts.alias == "_" && a.packageNameToken().Src() != "main" && !strings.HasSuffix(a.filename, "_test.go") && !importHasComment(a.tree, spec) {
		a.Report(spec, "blank import should be justified by a comment")
	}
}

func (a *Pass) checkImportAliasNaming(spec *cst.ImportSpec) {
	facts, ok := parseImport(spec)
	if ok && facts.alias != "" && facts.alias != "." && facts.alias != "_" && !validPackagePattern.MatchString(facts.alias) {
		a.Report(facts.aliasNode, "import alias should contain lower-case letters and digits")
	}
}

func (a *Pass) checkRedundantImportAlias(spec *cst.ImportSpec) {
	facts, ok := parseImport(spec)
	if ok && facts.alias != "" && facts.alias != "." && facts.alias != "_" && facts.alias == filepath.Base(facts.path) {
		a.Report(facts.aliasNode, "import alias is identical to the package name")
	}
}

func importHasComment(tree *cst.Tree, spec *cst.ImportSpec) bool {
	start, end := tree.Range(spec)
	startPosition := tree.Position(start)
	endPosition := tree.Position(end)
	source := tree.Bytes()
	for _, comment := range tree.Comments() {
		if comment.Line == endPosition.Line || (comment.End <= start && strings.Count(string(source[comment.End:start]), "\n") <= 1 && comment.Line+1 >= startPosition.Line) {
			return true
		}
	}
	return false
}

func (a *Pass) checkFileLength() {
	lines := bytes.Split(a.content, []byte("\n"))
	if limit := a.intOption("max-lines"); limit > 0 && len(lines) > limit {
		a.ReportRange(0, len(a.content), fmt.Sprintf("file has %d lines; maximum is %d", len(lines), limit))
	}
}

func (a *Pass) checkBidirectionalControlCharacters() {
	for offset := 0; offset < len(a.content); {
		character, width := utf8.DecodeRune(a.content[offset:])
		if width == 0 {
			break
		}
		if bidirectionalControl(character) {
			a.ReportRange(offset, offset+width, fmt.Sprintf("source contains invisible bidirectional control character U+%04X", character))
		}
		offset += width
	}
}

func (a *Pass) checkCompilerDirectiveSpacing() {
	for _, comment := range a.tree.Comments() {
		if comment.Column != 1 || !strings.HasPrefix(comment.Text, "//") {
			continue
		}
		body := comment.Text[2:]
		trimmed := strings.TrimLeft(body, " \t")
		name := strings.TrimPrefix(trimmed, "go:")
		if len(trimmed) != len(body) && strings.HasPrefix(trimmed, "go:") && name != "" && name[0] >= 'a' && name[0] <= 'z' {
			a.ReportRange(comment.Start, comment.End, "compiler directive is ignored because whitespace follows //")
		}
	}
}

func (a *Pass) checkPackageComment() {
	if strings.HasSuffix(a.filename, "_test.go") {
		return
	}
	nameToken := a.packageNameToken()
	packageStart, _ := a.tree.Range(nameToken)
	for _, comment := range a.tree.Comments() {
		if comment.End >= packageStart {
			break
		}
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(comment.Text, "//"), "/*"))
		text = strings.TrimSpace(strings.TrimSuffix(text, "*/"))
		if strings.HasPrefix(text, "Package "+nameToken.Src()) || strings.HasPrefix(strings.TrimPrefix(text, "Package "), nameToken.Src()) {
			return
		}
	}
	a.Report(nameToken, "package should have a documentation comment")
}

func (a *Pass) checkBuildTags() {
	comments := a.tree.Comments()
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
			a.ReportRange(comment.Start, comment.End, "legacy +build line is redundant with go:build")
		}
	}
	for index, comment := range plusBuild {
		constraint := legacyBuildTerms(comment.Text)
		for earlier := range index {
			if slices.Equal(constraint, legacyBuildTerms(plusBuild[earlier].Text)) {
				a.ReportRange(comment.Start, comment.End, "legacy build constraint duplicates an earlier constraint")
				break
			}
		}
	}
}
