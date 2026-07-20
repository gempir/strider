package semantic

import (
	"strings"
	"unicode"

	"github.com/gempir/strider/internal/diagnostic"
)

type taskCommentCheck struct{}

func (taskCommentCheck) Meta() Meta {
	return Meta{
		Code:            "task-comment",
		Summary:         "surface TODO, FIXME, and BUG comments",
		Explanation:     "Task markers in source are easy to forget and invisible to normal issue tracking. Resolve the task or link it to an owned work item before enforcing this advisory check.",
		GoodExample:     "// Retry only errors classified as transient.",
		BadExample:      "// TODO: decide which errors should be retried.",
		DefaultSeverity: diagnostic.SeverityNote,
	}
}

func (taskCommentCheck) Run(pass *Pass) {
	for _, file := range pass.Files {
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if marker := taskMarker(comment.Text); marker != "" {
					pass.Report(comment, marker+" comment should be resolved or linked to an owned work item")
				}
			}
		}
	}
}

func taskMarker(text string) string {
	fields := strings.FieldsFunc(text, func(character rune) bool {
		return !unicode.IsLetter(character)
	})
	for _, field := range fields {
		switch field {
		case "TODO", "FIXME", "BUG":
			return field
		}
	}
	return ""
}

func (taskCommentCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
