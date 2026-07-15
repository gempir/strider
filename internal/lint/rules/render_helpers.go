package rules

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"unicode"
	"unicode/utf8"
)

type positionedNode struct {
	start, end token.Pos
}

func (n positionedNode) Pos() token.Pos {
	return n.start
}

func (n positionedNode) End() token.Pos {
	return n.end
}

func exprText(expr ast.Expr) string {
	var buffer bytes.Buffer
	if expr != nil && format.Node(&buffer, token.NewFileSet(), expr) == nil {
		return buffer.String()
	}
	return ""
}

func nodeText(node ast.Node) string {
	var buffer bytes.Buffer
	if node != nil && format.Node(&buffer, token.NewFileSet(), node) == nil {
		return buffer.String()
	}
	return ""
}

func foldedExported(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}
