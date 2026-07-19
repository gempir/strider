package formatter

import (
	"bytes"
	"errors"
	"go/token"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

type syntaxFingerprint struct {
	imports []string
	syntax  []byte
}

func equivalentTrees(originalTree, formattedTree *cst.Tree) error {
	original := concreteFingerprint(originalTree)
	formatted := concreteFingerprint(formattedTree)
	if !slices.Equal(original.imports, formatted.imports) || !bytes.Equal(original.syntax, formatted.syntax) {
		return errors.New("formatted output changed the concrete syntax tree")
	}
	originalComments := originalTree.Comments()
	formattedComments := formattedTree.Comments()
	if len(originalComments) != len(formattedComments) {
		return errors.New("formatted output changed comment contents")
	}
	originalCommentText := make([]string, len(originalComments))
	formattedCommentText := make([]string, len(formattedComments))
	for index, comment := range originalComments {
		originalCommentText[index] = normalizeLineComment(comment.Text)
		formattedCommentText[index] = formattedComments[index].Text
	}
	sort.Strings(originalCommentText)
	sort.Strings(formattedCommentText)
	for index := range originalCommentText {
		if originalCommentText[index] != formattedCommentText[index] {
			return errors.New("formatted output changed comment contents")
		}
	}
	return nil
}

func concreteFingerprint(tree *cst.Tree) syntaxFingerprint {
	imports := []string{}
	output := make([]byte, 0, len(tree.Bytes()))
	var visit func(cst.Node)
	visit = func(node cst.Node) {
		if cst.Kind(node) == "TopLevelDeclList" {
			declarations := concreteTopLevelDeclarations(node)
			sort.SliceStable(
				declarations,
				func(left, right int) bool {
					return concreteDeclarationRank(declarations[left]) < concreteDeclarationRank(declarations[right])
				},
			)
			for _, declaration := range declarations {
				visit(declaration)
			}
			return
		}
		if cst.Kind(node) == "ImportDeclList" {
			for _, child := range cst.Children(node) {
				visit(child)
			}
			return
		}
		if declaration, ok := node.(*cst.ImportDecl); ok {
			cst.Walk(
				declaration,
				func(child cst.Node) bool {
					spec,
						isSpec := child.(*cst.ImportSpec)
					if !isSpec {
						return true
					}
					name := ""
					switch {
					case spec.PERIOD.IsValid():
						name = spec.PERIOD.Src()
					case spec.PackageName.IsValid():
						name = spec.PackageName.Src()
					}
					imports = append(imports, name+"\x00"+spec.ImportPath.Src())
					return false
				},
			)
			return
		}
		if current, ok := node.(cst.Token); ok {
			if current.Ch() != token.SEMICOLON && current.Ch() != token.COMMA {
				output = append(output, 'T')
				output = strconv.AppendInt(output, int64(current.Ch()), 10)
				output = append(output, ':')
				output = strconv.AppendQuote(output, current.Src())
				output = append(output, ';')
			}
			return
		}
		output = append(output, '(')
		output = append(output, strings.TrimRight(cst.Kind(node), "0123456789")...)
		for _, child := range cst.Children(node) {
			visit(child)
		}
		output = append(output, ')')
	}
	visit(tree.Root())
	sort.Strings(imports)
	return syntaxFingerprint{
		imports: imports,
		syntax:  output,
	}
}

func concreteTopLevelDeclarations(node cst.Node) []cst.Node {
	declarations := []cst.Node{}
	for _, child := range cst.Children(node) {
		if cst.Kind(child) == "TopLevelDeclList" {
			declarations = append(declarations, concreteTopLevelDeclarations(child)...)
			continue
		}
		if _, isToken := child.(cst.Token); !isToken {
			declarations = append(declarations, child)
		}
	}
	return declarations
}

func concreteDeclarationRank(node cst.Node) int {
	for _, current := range cst.NodeTokens(node) {
		switch current.Ch() {
		case token.CONST:
			return 0
		case token.VAR:
			return 1
		case token.TYPE:
			return 2
		case token.FUNC:
			return 3
		}
	}
	return 4
}
