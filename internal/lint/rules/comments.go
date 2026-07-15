package rules

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"strings"
	"unicode/utf8"
)

func (a *analyzer) checkLinesAndComments() {
	lines := bytes.Split(a.content, []byte("\n"))
	if a.on("line-length-limit") {
		file := a.fset.File(a.file.Pos())
		for index, line := range lines {
			if utf8.RuneCount(line) <= 80 || file == nil {
				continue
			}
			position := file.LineStart(index + 1)
			a.report(
				"line-length-limit",
				positionedNode{position, position + token.Pos(len(line))},
				fmt.Sprintf("line has %d runes; maximum is 80", utf8.RuneCount(line)),
			)
		}
	}
	if a.on("comment-spacings") {
		for _, group := range a.file.Comments {
			for _, comment := range group.List {
				text := comment.Text
				if strings.HasPrefix(text, "//") && len(text) > 2 && text[2] != ' ' &&
					text[2] != '\t' &&
					!commentDirective(text[2:]) {
					a.report(
						"comment-spacings",
						comment,
						"line comment should have a space after //",
					)
				}
			}
		}
	}
	if a.on("package-comments") {
		if !strings.HasSuffix(a.filename, "_test.go") {
			doc := a.file.Doc
			if doc == nil ||
				!strings.HasPrefix(
					strings.TrimSpace(strings.TrimPrefix(doc.Text(), "Package ")),
					a.file.Name.Name,
				) &&
					!strings.HasPrefix(doc.Text(), "Package "+a.file.Name.Name) {
				a.report(
					"package-comments",
					a.file.Name,
					"package should have a documentation comment",
				)
			}
		}
	}
	if a.on("redundant-build-tag") {
		hasGoBuild, plusBuild := false, []*ast.Comment{}
		for _, group := range a.file.Comments {
			for _, comment := range group.List {
				hasGoBuild = hasGoBuild || strings.HasPrefix(comment.Text, "//go:build")
				if strings.HasPrefix(comment.Text, "// +build") {
					plusBuild = append(plusBuild, comment)
				}
			}
		}
		if hasGoBuild {
			for _, comment := range plusBuild {
				a.report(
					"redundant-build-tag",
					comment,
					"legacy +build line is redundant with go:build",
				)
			}
		}
	}
}

func commentDirective(text string) bool {
	return strings.HasPrefix(text, "go:") || strings.HasPrefix(text, "line ") ||
		strings.HasPrefix(text, "+build") ||
		strings.HasPrefix(text, "nolint") ||
		strings.HasPrefix(text, "strider:") ||
		strings.HasPrefix(text, "Code generated") ||
		strings.HasPrefix(text, "TODO") ||
		strings.HasPrefix(text, "FIXME") ||
		strings.HasPrefix(text, "#")
}
