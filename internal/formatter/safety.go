//strider:ignore-file cognitive-complexity
package formatter

import (
	"bytes"
	"errors"
	"fmt"
	goformat "go/format"
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
	canonicalOriginal, err := goformat.Source(originalTree.Bytes())
	if err != nil {
		return fmt.Errorf("canonicalize original comments: %w", err)
	}
	canonicalTree, err := cst.Parse("formatter-safety.go", canonicalOriginal)
	if err != nil {
		return fmt.Errorf("parse canonical comments: %w", err)
	}
	originalComments := commentContentsForSafety(canonicalTree.Comments())
	formattedComments := commentContentsForSafety(formattedTree.Comments())
	if len(originalComments) != len(formattedComments) {
		return errors.New("formatted output changed comment contents")
	}
	for index, comment := range originalComments {
		if comment != formattedComments[index] {
			return errors.New("formatted output changed comment contents")
		}
	}
	return nil
}

func commentContentsForSafety(comments []cst.Comment) []string {
	result := make([]string, 0, len(comments))
	for _, comment := range comments {
		normalized := normalizeCommentForSafety(comment.Text)
		if strings.HasPrefix(normalized, "//") {
			body := strings.TrimSpace(strings.TrimPrefix(normalized, "//"))
			body = strings.TrimPrefix(body, "# ")
			if body == "" {
				continue
			}
			normalized = "// " + body
		}
		result = append(result, normalized)
	}
	return result
}

func normalizeCommentForSafety(comment string) string {
	if strings.HasPrefix(comment, "//") {
		return normalizeLineComment(comment)
	}
	lines := strings.Split(strings.ReplaceAll(comment, "\r\n", "\n"), "\n")
	if len(lines) < 2 {
		return comment
	}
	commonIndent := -1
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if commonIndent < 0 || indent < commonIndent {
			commonIndent = indent
		}
	}
	if commonIndent <= 0 {
		return strings.Join(lines, "\n")
	}
	for index := 1; index < len(lines); index++ {
		lines[index] = lines[index][min(commonIndent, len(lines[index])):]
	}
	return strings.Join(lines, "\n")
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
