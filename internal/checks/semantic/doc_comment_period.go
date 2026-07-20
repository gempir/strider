package semantic

import (
	"go/ast"
	"strings"

	"github.com/gempir/strider/internal/diagnostic"
)

type docCommentPeriodCheck struct{}

func (docCommentPeriodCheck) Meta() Meta {
	return Meta{
		Code:            "doc-comment-period",
		Summary:         "require declaration documentation to end with punctuation",
		Explanation:     "Complete documentation sentences are easier to read in generated API references. Documentation attached to packages, exported declarations, and exported specs should end with terminal punctuation.",
		GoodExample:     "// Client sends requests.",
		BadExample:      "// Client sends requests",
		DefaultSeverity: diagnostic.SeverityNote,
	}
}

func (docCommentPeriodCheck) Run(pass *Pass) {
	for _, file := range pass.Files {
		reported := make(map[*ast.CommentGroup]bool)
		reportDocCommentPeriod(pass, file.Doc, reported)
		for _, declaration := range file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Name.IsExported() {
					reportDocCommentPeriod(pass, declaration.Doc, reported)
				}
			case *ast.GenDecl:
				for _, raw := range declaration.Specs {
					switch spec := raw.(type) {
					case *ast.TypeSpec:
						if spec.Name.IsExported() {
							doc := spec.Doc
							if doc == nil {
								doc = declaration.Doc
							}
							reportDocCommentPeriod(pass, doc, reported)
						}
					case *ast.ValueSpec:
						exported := false
						for _, name := range spec.Names {
							exported = exported || name.IsExported()
						}
						if exported {
							doc := spec.Doc
							if doc == nil {
								doc = declaration.Doc
							}
							reportDocCommentPeriod(pass, doc, reported)
						}
					}
				}
			}
		}
	}
}

func reportDocCommentPeriod(pass *Pass, group *ast.CommentGroup, reported map[*ast.CommentGroup]bool) {
	if group == nil || reported[group] {
		return
	}
	reported[group] = true
	text := strings.TrimSpace(group.Text())
	if text == "" {
		return
	}
	last := text[len(text)-1]
	if last == '.' || last == '!' || last == '?' || last == ':' {
		return
	}
	pass.Report(group, "documentation comment should end with punctuation")
}

func (docCommentPeriodCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
