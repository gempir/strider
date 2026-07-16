package rules

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"
	"unicode/utf8"
)

func (a *analyzer) checkLinesAndComments() {
	lines := bytes.Split(a.content, []byte("\n"))
	if a.on("bidirectional-control-character") {
		file := a.fset.File(a.file.Pos())
		for offset := 0; offset < len(a.content); {
			character, width := utf8.DecodeRune(a.content[offset:])
			if width == 0 {
				break
			}
			if file != nil && bidirectionalControl(character) {
				position := file.Pos(offset)
				a.report(
					"bidirectional-control-character",
					positionedNode{position, position + token.Pos(width)},
					fmt.Sprintf("source contains invisible bidirectional control character U+%04X", character),
				)
			}
			offset += width
		}
	}
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
	if a.on("spaced-compiler-directive") {
		for _, group := range a.file.Comments {
			for _, comment := range group.List {
				if !strings.HasPrefix(comment.Text, "//") ||
					a.fset.Position(comment.Pos()).Column != 1 {
					continue
				}
				body := comment.Text[2:]
				trimmed := strings.TrimLeft(body, " \t")
				if len(trimmed) == len(body) || !strings.HasPrefix(trimmed, "go:") {
					continue
				}
				name := strings.TrimPrefix(trimmed, "go:")
				if name == "" || name[0] < 'a' || name[0] > 'z' {
					continue
				}
				a.report(
					"spaced-compiler-directive",
					comment,
					"compiler directive is ignored because whitespace follows //",
				)
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
		for index, comment := range plusBuild {
			constraint := legacyBuildTerms(comment.Text)
			for earlier := range index {
				if slices.Equal(constraint, legacyBuildTerms(plusBuild[earlier].Text)) {
					a.report(
						"redundant-build-tag",
						comment,
						"legacy build constraint duplicates an earlier constraint",
					)
					break
				}
			}
		}
	}
}

func bidirectionalControl(character rune) bool {
	switch character {
	case '\u202a', '\u202b', '\u202c', '\u202d', '\u202e',
		'\u2066', '\u2067', '\u2068', '\u2069':
		return true
	default:
		return false
	}
}

func legacyBuildTerms(text string) []string {
	terms := strings.Fields(strings.TrimSpace(strings.TrimPrefix(text, "// +build")))
	slices.Sort(terms)
	return terms
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
