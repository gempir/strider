package rules

import (
	"bytes"
	"go/token"
	"strings"
	"unicode"

	"github.com/gempir/strider/internal/cst"
)

type documentationPeriodState struct {
	reported map[int]bool
}

func (a *Pass) checkTaskComments() {
	for _, comment := range a.tree.Comments() {
		if marker := taskMarker(comment.Text); marker != "" {
			a.reportRange("task-comment", comment.Start, comment.End, marker+" comment should be resolved or linked to an owned work item")
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

func (a *Pass) checkDocumentationPeriod(node cst.Node) {
	if node == nil {
		a.checkDocumentationComment(a.packageNameToken())
		return
	}
	if !exportedDeclaration(node) {
		return
	}
	comment, ok := a.attachedComment(node)
	if !ok {
		for index := len(a.ancestors) - 1; index >= 0; index-- {
			switch a.ancestors[index].(type) {
			case *cst.ConstDecl, *cst.VarDecl, *cst.TypeDecl:
				comment, ok = a.attachedComment(a.ancestors[index])
			}
			if ok {
				break
			}
		}
	}
	if ok {
		a.reportDocumentationComment(comment)
	}
}

func (a *Pass) checkDocumentationComment(node cst.Node) {
	if comment, ok := a.attachedComment(node); ok {
		a.reportDocumentationComment(comment)
	}
}

func (a *Pass) reportDocumentationComment(comment cst.Comment) {
	state := checkState(a, func() *documentationPeriodState {
		return &documentationPeriodState{
			reported: make(map[int]bool),
		}
	})
	if state.reported[comment.Start] {
		return
	}
	state.reported[comment.Start] = true
	text := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(comment.Text, "//"), "/*"))
	text = strings.TrimSpace(strings.TrimSuffix(text, "*/"))
	if text == "" {
		return
	}
	switch text[len(text)-1] {
	case '.', '!', '?', ':':
		return
	}
	a.reportRange("doc-comment-period", comment.Start, comment.End, "documentation comment should end with punctuation")
}

func (a *Pass) attachedComment(node cst.Node) (cst.Comment, bool) {
	start, _ := cst.Range(node)
	comments := a.tree.Comments()
	for index := len(comments) - 1; index >= 0; index-- {
		comment := comments[index]
		if comment.End > start {
			continue
		}
		gap := a.content[comment.End:start]
		if len(bytes.TrimSpace(gap)) != 0 || bytes.Count(gap, []byte("\n")) > 1 {
			return cst.Comment{}, false
		}
		return comment, true
	}
	return cst.Comment{}, false
}

func exportedDeclaration(node cst.Node) bool {
	switch current := node.(type) {
	case *cst.FunctionDecl:
		return current.FunctionName != nil && token.IsExported(current.FunctionName.IDENT.Src())
	case *cst.MethodDecl:
		return token.IsExported(current.MethodName.Src())
	case *cst.VarSpec:
		return token.IsExported(current.IDENT.Src())
	case *cst.VarSpec2:
		return exportedIdentifierList(current.IdentifierList)
	case *cst.ConstSpec:
		return token.IsExported(current.IDENT.Src())
	case *cst.ConstSpec2:
		return exportedIdentifierList(current.IdentifierList)
	case *cst.TypeDef:
		return token.IsExported(current.IDENT.Src())
	case *cst.AliasDecl:
		return token.IsExported(current.IDENT.Src())
	default:
		return false
	}
}

func exportedIdentifierList(list *cst.IdentifierList) bool {
	for ; list != nil; list = list.List {
		if token.IsExported(list.IDENT.Src()) {
			return true
		}
	}
	return false
}

func (a *Pass) checkTopLevelDeclarationOrder(file *cst.SourceFile) {
	highest := -1
	for list := file.TopLevelDeclList; list != nil; list = list.List {
		rank := syntaxDeclarationRank(list.TopLevelDecl)
		if rank < highest {
			a.report("top-level-declaration-order", list.TopLevelDecl, "top-level declarations should be ordered as const, var, type, then func")
			return
		}
		if rank > highest {
			highest = rank
		}
	}
}

func syntaxDeclarationRank(node cst.Node) int {
	switch node.(type) {
	case *cst.ConstDecl:
		return 0
	case *cst.VarDecl:
		return 1
	case *cst.TypeDecl:
		return 2
	case *cst.FunctionDecl, *cst.MethodDecl:
		return 3
	default:
		return -1
	}
}

func (a *Pass) checkExcessiveBlankIdentifiers(node cst.Node) {
	blanks := 0
	switch current := node.(type) {
	case *cst.Assignment:
		for list := current.ExpressionList; list != nil; list = list.List {
			if cst.Spelling(list.Expression) == "_" {
				blanks++
			}
		}
	case *cst.ShortVarDecl:
		for list := current.IdentifierList; list != nil; list = list.List {
			if list.IDENT.Src() == "_" {
				blanks++
			}
		}
	}
	if blanks >= 3 {
		a.report("excessive-blank-identifiers", node, "assignment discards three or more results; name meaningful results or simplify the return contract")
	}
}
