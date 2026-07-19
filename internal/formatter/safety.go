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

func equivalentTrees(originalTree, formattedTree *cst.Tree) error {
	original := concreteFingerprint(originalTree)
	formatted := concreteFingerprint(formattedTree)
	if !slices.Equal(original.imports, formatted.imports) || !bytes.Equal(original.syntax, formatted.syntax) {
		return errors.New("formatted output changed the concrete syntax tree")
	}
	originalComments := originalTree.Comments()
	formattedComments := formattedTree.Comments()
	if len(originalComments) != len(formattedComments) {
		return errors.New("formatted output changed comment contents or ordering")
	}
	for index, comment := range originalComments {
		if normalizeLineComment(comment.Text) != formattedComments[index].Text {
			return errors.New("formatted output changed comment contents or ordering")
		}
	}
	return nil
}

type syntaxFingerprint struct {
	imports []string
	syntax  []byte
}

func concreteFingerprint(tree *cst.Tree) syntaxFingerprint {
	imports := []string{}
	output := make([]byte, 0, len(tree.Bytes()))
	var visit func(cst.Node)
	visit = func(node cst.Node) {
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
	return syntaxFingerprint{imports: imports, syntax: output}
}
