package formatter

import (
	"errors"
	"fmt"
	"go/token"
	"sort"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

func equivalent(filename string, original, formatted []byte) error {
	originalTree, err := cst.Parse(filename, original)
	if err != nil {
		return err
	}
	formattedTree, err := cst.Parse(filename, formatted)
	if err != nil {
		return fmt.Errorf("formatted output does not parse: %w", err)
	}
	if concreteFingerprint(originalTree) != concreteFingerprint(formattedTree) {
		return errors.New("formatted output changed the concrete syntax tree")
	}
	if concreteCommentFingerprint(originalTree) != concreteCommentFingerprint(formattedTree) {
		return errors.New("formatted output changed comment contents or ordering")
	}
	return nil
}

func concreteFingerprint(tree *cst.Tree) string {
	imports := []string{}
	var output strings.Builder
	var visit func(cst.Node)
	visit = func(node cst.Node) {
		if cst.Kind(node) == "ImportDeclList" {
			for _, child := range cst.Children(node) {
				visit(child)
			}
			return
		}
		if declaration, ok := node.(*cst.ImportDecl); ok {
			cst.Walk(declaration, func(child cst.Node) bool {
				spec, isSpec := child.(*cst.ImportSpec)
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
			})
			return
		}
		if current, ok := node.(cst.Token); ok {
			if current.Ch() != token.SEMICOLON && current.Ch() != token.COMMA {
				fmt.Fprintf(&output, "T%d:%q;", current.Ch(), current.Src())
			}
			return
		}
		output.WriteByte('(')
		output.WriteString(strings.TrimRight(cst.Kind(node), "0123456789"))
		for _, child := range cst.Children(node) {
			visit(child)
		}
		output.WriteByte(')')
	}
	visit(tree.Root())
	sort.Strings(imports)
	return "imports:" + strings.Join(imports, "\x01") + "\n" + output.String()
}

func concreteCommentFingerprint(tree *cst.Tree) string {
	comments := tree.Comments()
	texts := make([]string, 0, len(comments))
	for _, comment := range comments {
		texts = append(texts, comment.Text)
	}
	return strings.Join(texts, "\x00")
}
