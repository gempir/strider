package formatter

import (
	"go/token"
	"strings"
	"unicode/utf8"
)

type writer struct {
	output          strings.Builder
	indent          int
	column          int
	pendingNewlines int
	forceSpace      bool
	lineStart       bool
	maxEmptyLines   int
}

func (w *writer) beforeToken(previous, current token.Token, force bool) {
	if w.lineStart || w.pendingNewlines != 0 {
		return
	}
	if force || w.forceSpace || needsSpace(previous, current, false) {
		w.output.WriteByte(' ')
		w.column++
	}
	w.forceSpace = false
}

func needsSpace(previous, current token.Token, spaced bool) bool {
	if previous == token.ILLEGAL || spaced {
		return spaced
	}
	if previous == token.LPAREN || previous == token.LBRACK || previous == token.PERIOD || current == token.RPAREN || current == token.RBRACK || current == token.COMMA || current == token.PERIOD || current == token.COLON || current == token.LPAREN || current == token.LBRACK {
		return false
	}
	if current == token.LBRACE {
		return true
	}
	if tokenWord(previous) && tokenWord(current) {
		return true
	}
	if (previous == token.RPAREN || previous == token.RBRACE) && tokenWord(current) {
		return true
	}
	return false
}

func tokenWord(kind token.Token) bool {
	return kind == token.IDENT || kind.IsKeyword() || kind == token.INT || kind == token.FLOAT || kind == token.IMAG || kind == token.CHAR || kind == token.STRING
}

func (w *writer) write(value string, indentOverride int) {
	if value == "" {
		return
	}
	w.flushNewlines()
	if w.lineStart {
		indent := w.indent
		if indentOverride >= 0 {
			indent = indentOverride
		}
		w.output.WriteString(strings.Repeat("\t", indent))
		w.column = indent * 8
		w.lineStart = false
	}
	if w.forceSpace && w.column != 0 {
		w.output.WriteByte(' ')
		w.column++
		w.forceSpace = false
	}
	w.output.WriteString(value)
	if newline := strings.LastIndexByte(value, '\n'); newline >= 0 {
		w.column = utf8.RuneCountInString(value[newline+1:])
		w.lineStart = newline == len(value)-1
		return
	}
	w.column += utf8.RuneCountInString(value)
}

func (w *writer) requestNewlines(count int) {
	w.forceSpace = false
	count = min(count, w.maxEmptyLines+1)
	if count > w.pendingNewlines {
		w.pendingNewlines = count
	}
}

func (w *writer) requestRequiredNewlines(count int) {
	w.forceSpace = false
	if count > w.pendingNewlines {
		w.pendingNewlines = count
	}
}

func (w *writer) preserveEmptyLines(source []byte, start, end int, buildConstraint bool) {
	if w.output.Len() == 0 {
		return
	}
	newlines := countNewlines(source, start, end)
	if newlines <= 1 {
		return
	}
	w.requestNewlines(newlines)
	if buildConstraint {
		w.requestRequiredNewlines(2)
	}
}

func (w *writer) flushNewlines() {
	if w.pendingNewlines == 0 {
		return
	}
	if w.lineStart && w.output.Len() != 0 {
		w.pendingNewlines--
	}
	for range w.pendingNewlines {
		w.output.WriteByte('\n')
	}
	w.pendingNewlines = 0
	w.lineStart = true
	w.column = 0
}
