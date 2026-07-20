package formatter

import (
	"go/token"
	"sort"
	"strconv"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

type importEntry struct {
	name string
	path string
}

func (l *layout) indexImports() {
	cst.Walk(
		l.tree.Root(),
		func(node cst.Node) bool {
			declaration, ok := node.(*cst.ImportDecl)
			if !ok {
				return true
			}
			tokens := cst.NodeTokens(declaration)
			if len(tokens) != 0 {
				start, _ := l.tokenIndex(tokens[0])
				end, _ := l.tokenIndex(tokens[len(tokens)-1])
				if l.importStart < 0 || start < l.importStart {
					l.importStart = start
				}
				if end > l.importEnd {
					l.importEnd = end
				}
			}
			for _, spec := range cst.ImportSpecs(declaration) {
				l.imports = append(l.imports, importEntry{
					name: importSpecName(spec),
					path: spec.ImportPath.Src(),
				})
			}
			return false
		},
	)
	if l.importStart < 0 || len(l.imports) == 0 {
		l.importStart, l.importEnd = -1, -1
		return
	}
	for l.importEnd+1 < len(l.tokens) && l.tokens[l.importEnd+1].Ch() == token.SEMICOLON {
		l.importEnd++
	}
	for _, comment := range l.tree.Comments() {
		start := l.tokens[l.importStart].Position().Offset
		end := l.tokens[l.importEnd].Position().Offset + len(l.tokens[l.importEnd].Src())
		if comment.Start >= start && comment.End <= end {
			l.importStart, l.importEnd = -1, -1
			return
		}
	}
}

func (l *layout) renderImports(writer *writer) {
	imports := append([]importEntry(nil), l.imports...)
	sort.SliceStable(
		imports,
		func(i, j int) bool {
			leftCategory := l.importCategory(imports[i])
			rightCategory := l.importCategory(imports[j])
			if leftCategory != rightCategory {
				return leftCategory < rightCategory
			}
			return imports[i].path < imports[j].path
		},
	)
	if len(imports) == 1 {
		writer.write("import "+importText(imports[0]), -1)
		writer.requestNewlines(2)
		return
	}
	writer.write("import (", -1)
	writer.indent++
	writer.requestNewlines(1)
	previousCategory := -1
	for _, current := range imports {
		category := l.importCategory(current)
		if previousCategory >= 0 && category != previousCategory {
			writer.requestNewlines(2)
		}
		writer.write(importText(current), -1)
		writer.requestNewlines(1)
		previousCategory = category
	}
	writer.indent--
	writer.write(")", -1)
	writer.requestNewlines(2)
}

func (l *layout) importCategory(item importEntry) int {
	path, err := strconv.Unquote(item.path)
	if err != nil {
		return 1
	}
	if l.module != "" && (path == l.module || strings.HasPrefix(path, l.module+"/")) {
		return 2
	}
	first := strings.Split(path, "/")[0]
	if !strings.Contains(first, ".") {
		return 0
	}
	return 1
}

func importText(item importEntry) string {
	if item.name == "" {
		return item.path
	}
	return item.name + " " + item.path
}

func importSpecName(spec *cst.ImportSpec) string {
	switch {
	case spec.PERIOD.IsValid():
		return spec.PERIOD.Src()
	case spec.PackageName.IsValid():
		return spec.PackageName.Src()
	default:
		return ""
	}
}
