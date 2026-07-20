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
	original := fingerprintTree(originalTree)
	formatted := fingerprintTree(formattedTree)
	if !slices.Equal(original.imports, formatted.imports) || !bytes.Equal(original.syntax, formatted.syntax) {
		return errors.New("formatted output changed the concrete syntax tree")
	}
	originalComments := originalTree.Comments()
	formattedComments := formattedTree.Comments()
	if len(originalComments) != len(formattedComments) {
		return errors.New("formatted output changed comment contents")
	}
	for index, comment := range originalComments {
		if normalizeLineComment(comment.Text) != formattedComments[index].Text {
			return errors.New("formatted output changed comment contents")
		}
	}
	return nil
}

func fingerprintTree(tree *cst.Tree) syntaxFingerprint {
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
			for _, spec := range cst.ImportSpecs(declaration) {
				imports = append(imports, importSpecName(spec)+"\x00"+spec.ImportPath.Src())
			}
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
