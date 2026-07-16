package formatter

import (
	"bufio"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gempir/strider/internal/cst"
)

type concreteTokenKey struct {
	offset int
	kind   token.Token
	source string
}

type concreteGroup struct {
	close  int
	broken bool
}

type concreteLayout struct {
	tree          *cst.Tree
	tokens        []cst.Token
	indices       map[concreteTokenKey]int
	hardOpen      map[int]int
	hardClose     map[int]bool
	softOpen      map[int]int
	softClose     map[int]bool
	softSemis     map[int]bool
	topSemis      map[int]bool
	spacedOps     map[int]bool
	spaceBefore   map[int]bool
	unaryOps      map[int]bool
	channelArrows map[int]bool
	spacedAfter   map[int]bool
	labelColons   map[int]bool
	caseTokens    map[int]bool
	caseColons    map[int]bool
	importStart   int
	importEnd     int
	imports       []concreteImport
	module        string
}

type concreteImport struct {
	name string
	path string
}

type concreteWriter struct {
	output          strings.Builder
	indent          int
	column          int
	pendingNewlines int
	forceSpace      bool
	lineStart       bool
	indentWidth     int
	maxEmptyLines   int
}

func renderConcrete(filename string, tree *cst.Tree, options Options) string {
	layout := newConcreteLayout(filename, tree)
	writer := concreteWriter{
		lineStart:     true,
		indentWidth:   options.IndentWidth,
		maxEmptyLines: options.MaxEmptyLines,
	}
	source := tree.Source()
	comments := tree.Comments()
	commentIndex := 0
	groups := []concreteGroup{}
	previous := token.ILLEGAL
	sourceEnd := 0

	for index := 0; index < len(layout.tokens); index++ {
		current := layout.tokens[index]
		if current.Ch() == token.EOF {
			layout.renderCommentsBefore(&writer, comments, &commentIndex, source, &sourceEnd, len(source)+1)
			break
		}
		position := current.Position()
		layout.renderCommentsBefore(&writer, comments, &commentIndex, source, &sourceEnd, position.Offset)

		if layout.importStart >= 0 && index == layout.importStart {
			layout.renderImports(&writer)
			index = layout.importEnd
			last := layout.tokens[layout.importEnd]
			sourceEnd = max(sourceEnd, concreteSourceEnd(last))
			previous = token.SEMICOLON
			continue
		}
		if layout.importStart >= 0 && index > layout.importStart && index <= layout.importEnd {
			continue
		}

		kind := current.Ch()
		switch {
		case kind == token.SEMICOLON:
			if layout.softSemis[index] {
				writer.write(";", -1)
				writer.forceSpace = true
			} else if layout.topSemis[index] {
				writer.requestNewlines(2)
			} else {
				writer.requestNewlines(1)
			}
		case layout.hardOpen[index] != 0:
			writer.beforeToken(previous, kind, true)
			writer.write(current.Src(), -1)
			if layout.hardOpen[index] != index+1 {
				writer.indent++
				writer.requestNewlines(1)
			}
		case layout.hardClose[index]:
			if index > 0 && layout.hardOpen[index-1] != index {
				writer.indent = max(0, writer.indent-1)
				writer.requestNewlines(1)
			}
			writer.write(current.Src(), -1)
		case layout.softOpen[index] != 0:
			if layout.spaceBefore[index] {
				writer.beforeToken(previous, kind, true)
			}
			writer.write(current.Src(), -1)
			close := layout.softOpen[index]
			broken := layout.shouldBreak(
				index,
				close,
				writer.column,
				options.PrintWidth,
				comments,
				source,
				options.MaxEmptyLines > 0,
			)
			groups = append(groups, concreteGroup{close: close, broken: broken})
			if broken && close != index+1 {
				writer.indent++
				writer.requestNewlines(1)
			}
		case layout.softClose[index]:
			group := concreteGroup{}
			if len(groups) != 0 {
				group = groups[len(groups)-1]
				groups = groups[:len(groups)-1]
			}
			if group.broken && group.close == index && previous != token.COMMA {
				writer.write(",", -1)
			}
			if group.broken && group.close == index {
				writer.indent = max(0, writer.indent-1)
				writer.requestNewlines(1)
			}
			writer.write(current.Src(), -1)
		case kind == token.COMMA:
			if len(groups) != 0 && !groups[len(groups)-1].broken &&
				groups[len(groups)-1].close == index+1 {
				break
			}
			writer.write(",", -1)
			if len(groups) != 0 && groups[len(groups)-1].broken {
				writer.requestNewlines(1)
			} else {
				writer.forceSpace = true
			}
		case layout.caseTokens[index]:
			writer.requestNewlines(1)
			writer.write(current.Src(), max(0, writer.indent-1))
			writer.forceSpace = kind == token.CASE
		case layout.caseColons[index]:
			writer.write(":", -1)
			writer.requestNewlines(1)
		case layout.labelColons[index]:
			writer.write(":", -1)
			writer.requestNewlines(1)
		case layout.channelArrows[index]:
			if previous == token.CHAN {
				writer.write(current.Src(), -1)
				writer.forceSpace = true
			} else {
				writer.beforeToken(previous, kind, true)
				writer.write(current.Src(), -1)
			}
		case layout.spacedOps[index]:
			writer.beforeToken(previous, kind, true)
			writer.write(current.Src(), -1)
			writer.forceSpace = true
		case layout.unaryOps[index]:
			writer.beforeToken(previous, kind, previous.IsKeyword())
			writer.write(current.Src(), -1)
		case kind == token.COLON:
			writer.write(":", -1)
			writer.forceSpace = layout.spacedAfter[index]
		case kind == token.PERIOD:
			writer.write(".", -1)
		default:
			writer.beforeToken(previous, kind, layout.spaceBefore[index])
			writer.write(current.Src(), -1)
		}
		previous = kind
		sourceEnd = max(sourceEnd, concreteSourceEnd(current))
	}

	result := strings.TrimRight(writer.output.String(), " \t\r\n") + "\n"
	if options.EndOfLine == "crlf" {
		result = strings.ReplaceAll(result, "\n", "\r\n")
	}
	return result
}

func concreteSourceEnd(current cst.Token) int {
	end := current.Position().Offset
	if current.Ch() != token.SEMICOLON || current.Src() == ";" {
		end += len(current.Src())
	}
	return end
}

func newConcreteLayout(filename string, tree *cst.Tree) *concreteLayout {
	tokens := tree.Tokens()
	layout := &concreteLayout{
		tree:          tree,
		tokens:        tokens,
		indices:       make(map[concreteTokenKey]int, len(tokens)),
		hardOpen:      make(map[int]int),
		hardClose:     make(map[int]bool),
		softOpen:      make(map[int]int),
		softClose:     make(map[int]bool),
		softSemis:     make(map[int]bool),
		topSemis:      make(map[int]bool),
		spacedOps:     make(map[int]bool),
		spaceBefore:   make(map[int]bool),
		unaryOps:      make(map[int]bool),
		channelArrows: make(map[int]bool),
		spacedAfter:   make(map[int]bool),
		labelColons:   make(map[int]bool),
		caseTokens:    make(map[int]bool),
		caseColons:    make(map[int]bool),
		importStart:   -1,
		importEnd:     -1,
		module:        findModulePath(filename),
	}
	for index, current := range tokens {
		layout.indices[layout.key(current)] = index
		switch current.Ch() {
		case token.ASSIGN, token.DEFINE, token.ADD_ASSIGN, token.SUB_ASSIGN,
			token.MUL_ASSIGN, token.QUO_ASSIGN, token.REM_ASSIGN, token.AND_ASSIGN,
			token.OR_ASSIGN, token.XOR_ASSIGN, token.SHL_ASSIGN, token.SHR_ASSIGN,
			token.AND_NOT_ASSIGN:
			layout.spacedOps[index] = true
		}
	}
	layout.indexTree()
	layout.indexImports()
	return layout
}

func (l *concreteLayout) indexTree() {
	cst.Walk(l.tree.Root(), func(node cst.Node) bool {
		kind := cst.Kind(node)
		switch kind {
		case "Block", "StructType", "InterfaceType", "ExprSwitchStmt", "TypeSwitchStmt", "SelectStmt":
			l.markDelimited(node, token.LBRACE, token.RBRACE, l.hardOpen, l.hardClose)
		case "ConstDecl", "TypeDecl", "VarDecl":
			l.markDelimited(node, token.LPAREN, token.RPAREN, l.hardOpen, l.hardClose)
		case "Parameters", "LiteralValue", "TypeParameters", "TypeArgs":
			l.markAnyDelimited(node, l.softOpen, l.softClose)
		default:
			if strings.HasPrefix(kind, "Arguments") {
				l.markDelimited(node, token.LPAREN, token.RPAREN, l.softOpen, l.softClose)
			}
		}
		switch kind {
		case "ForClause", "IfElseStmt", "IfStmt", "ExprSwitchStmt", "TypeSwitchStmt":
			l.markTokens(node, token.SEMICOLON, l.softSemis)
		case "SourceFile", "ImportDeclList", "TopLevelDeclList":
			l.markTokens(node, token.SEMICOLON, l.topSemis)
		}
		switch current := node.(type) {
		case *cst.BinaryExpression:
			l.markToken(current.Op, l.spacedOps)
		case *cst.Assignment:
			l.markToken(current.Op, l.spacedOps)
		case *cst.ShortVarDecl:
			l.markToken(current.DEFINE, l.spacedOps)
		case *cst.UnaryExpr:
			l.markToken(current.Op, l.unaryOps)
		case *cst.ParameterDecl:
			if current.IdentifierList != nil {
				l.markFirstToken(current.TypeNode, l.spaceBefore)
			}
		case *cst.VarSpec:
			if current.TypeNode != nil {
				l.markFirstToken(current.TypeNode, l.spaceBefore)
			}
		case *cst.VarSpec2:
			if current.TypeNode != nil {
				l.markFirstToken(current.TypeNode, l.spaceBefore)
			}
		case *cst.FieldDecl:
			if current.IdentifierList != nil && current.TypeNode != nil {
				l.markFirstToken(current.TypeNode, l.spaceBefore)
			}
		case *cst.Result:
			if current.Parameters != nil {
				l.markFirstToken(current.Parameters, l.spaceBefore)
			} else if current.TypeNode != nil {
				l.markFirstToken(current.TypeNode, l.spaceBefore)
			}
		case *cst.MethodDecl:
			if current.Receiver != nil {
				l.markFirstToken(current.Receiver, l.spaceBefore)
			}
		case *cst.TypeDef:
			l.markFirstToken(current.TypeNode, l.spaceBefore)
		case *cst.TypeParamDecl:
			l.markFirstToken(current.TypeConstraint, l.spaceBefore)
		case *cst.TypeElemList:
			l.markToken(current.OR, l.spacedOps)
		}
		if kind == "SendStmt" || kind == "RangeClause" || kind == "TypeSwitchGuard" {
			for _, current := range cst.NodeTokens(node) {
				switch current.Ch() {
				case token.ARROW, token.ASSIGN, token.DEFINE:
					l.markToken(current, l.spacedOps)
				}
			}
		}
		if kind == "RecvStmt" {
			for _, current := range cst.NodeTokens(node) {
				if current.Ch() == token.ASSIGN || current.Ch() == token.DEFINE {
					l.markToken(current, l.spacedOps)
				}
			}
		}
		if kind == "ChannelType" {
			l.markTokens(node, token.ARROW, l.channelArrows)
		}
		if kind == "KeyedElement" {
			l.markTokens(node, token.COLON, l.spacedAfter)
		}
		if kind == "LabeledStmt" {
			l.markTokens(node, token.COLON, l.labelColons)
		}
		if kind == "ExprCaseClauseList" || kind == "TypeCaseClause" || kind == "CommClause" {
			for _, current := range cst.NodeTokens(node) {
				switch current.Ch() {
				case token.CASE, token.DEFAULT:
					l.markToken(current, l.caseTokens)
				case token.COLON:
					l.markToken(current, l.caseColons)
				}
			}
		}
		return true
	})
}

func (l *concreteLayout) markAnyDelimited(node cst.Node, opens map[int]int, closes map[int]bool) {
	tokens := cst.NodeTokens(node)
	if len(tokens) < 2 {
		return
	}
	open, close := tokens[0].Ch(), tokens[len(tokens)-1].Ch()
	if (open == token.LPAREN && close == token.RPAREN) ||
		(open == token.LBRACK && close == token.RBRACK) ||
		(open == token.LBRACE && close == token.RBRACE) {
		l.markDelimited(node, open, close, opens, closes)
	}
}

func (l *concreteLayout) markDelimited(
	node cst.Node,
	openKind, closeKind token.Token,
	opens map[int]int,
	closes map[int]bool,
) {
	open, close := -1, -1
	for _, child := range cst.Children(node) {
		current, ok := child.(cst.Token)
		if !ok {
			continue
		}
		index, ok := l.tokenIndex(current)
		if !ok {
			continue
		}
		if open < 0 && current.Ch() == openKind {
			open = index
		}
		if current.Ch() == closeKind {
			close = index
		}
	}
	if open >= 0 && close > open {
		opens[open] = close
		closes[close] = true
	}
}

func (l *concreteLayout) markTokens(node cst.Node, wanted token.Token, target map[int]bool) {
	for _, child := range cst.Children(node) {
		current, ok := child.(cst.Token)
		if !ok {
			continue
		}
		if current.Ch() == wanted {
			l.markToken(current, target)
		}
	}
}

func (l *concreteLayout) markToken(current cst.Token, target map[int]bool) {
	if index, ok := l.tokenIndex(current); ok {
		target[index] = true
	}
}

func (l *concreteLayout) markFirstToken(node cst.Node, target map[int]bool) {
	if node == nil {
		return
	}
	tokens := cst.NodeTokens(node)
	if len(tokens) != 0 {
		l.markToken(tokens[0], target)
	}
}

func (l *concreteLayout) tokenIndex(current cst.Token) (int, bool) {
	index, ok := l.indices[l.key(current)]
	return index, ok
}

func (l *concreteLayout) key(current cst.Token) concreteTokenKey {
	if !current.IsValid() {
		return concreteTokenKey{}
	}
	return concreteTokenKey{
		offset: current.Position().Offset,
		kind:   current.Ch(),
		source: current.Src(),
	}
}

func (l *concreteLayout) indexImports() {
	cst.Walk(l.tree.Root(), func(node cst.Node) bool {
		declaration, ok := node.(*cst.ImportDecl)
		if !ok {
			return true
		}
		tokens := cst.NodeTokens(declaration)
		if len(tokens) != 0 {
			start, _ := l.tokenIndex(tokens[0])
			end, _ := l.tokenIndex(tokens[len(tokens)-1])
			if l.importStart < 0 || start < l.importStart {
				l.importStart = start
			}
			if end > l.importEnd {
				l.importEnd = end
			}
		}
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
			l.imports = append(l.imports, concreteImport{name: name, path: spec.ImportPath.Src()})
			return false
		})
		return false
	})
	if l.importStart < 0 || len(l.imports) == 0 {
		l.importStart, l.importEnd = -1, -1
		return
	}
	for l.importEnd+1 < len(l.tokens) && l.tokens[l.importEnd+1].Ch() == token.SEMICOLON {
		l.importEnd++
	}
	for _, comment := range l.tree.Comments() {
		start := l.tokens[l.importStart].Position().Offset
		end := l.tokens[l.importEnd].Position().Offset + len(l.tokens[l.importEnd].Src())
		if comment.Start >= start && comment.End <= end {
			l.importStart, l.importEnd = -1, -1
			return
		}
	}
}

func (l *concreteLayout) renderImports(writer *concreteWriter) {
	imports := append([]concreteImport(nil), l.imports...)
	sort.SliceStable(imports, func(i, j int) bool {
		leftCategory := l.importCategory(imports[i])
		rightCategory := l.importCategory(imports[j])
		if leftCategory != rightCategory {
			return leftCategory < rightCategory
		}
		return imports[i].path < imports[j].path
	})
	if len(imports) == 1 {
		writer.write("import "+importText(imports[0]), -1)
		writer.requestNewlines(2)
		return
	}
	writer.write("import (", -1)
	writer.indent++
	writer.requestNewlines(1)
	previousCategory := -1
	for _, current := range imports {
		category := l.importCategory(current)
		if previousCategory >= 0 && category != previousCategory {
			writer.requestNewlines(2)
		}
		writer.write(importText(current), -1)
		writer.requestNewlines(1)
		previousCategory = category
	}
	writer.indent--
	writer.write(")", -1)
	writer.requestNewlines(2)
}

func (l *concreteLayout) importCategory(item concreteImport) int {
	path, err := strconv.Unquote(item.path)
	if err != nil {
		return 1
	}
	if l.module != "" && (path == l.module || strings.HasPrefix(path, l.module+"/")) {
		return 2
	}
	first := strings.Split(path, "/")[0]
	if !strings.Contains(first, ".") {
		return 0
	}
	return 1
}

func importText(item concreteImport) string {
	if item.name == "" {
		return item.path
	}
	return item.name + " " + item.path
}

func findModulePath(filename string) string {
	if filename == "" || strings.HasPrefix(filename, "<") {
		return ""
	}
	directory, err := filepath.Abs(filepath.Dir(filename))
	if err != nil {
		return ""
	}
	for {
		file, err := os.Open(filepath.Join(directory, "go.mod"))
		if err == nil {
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				fields := strings.Fields(scanner.Text())
				if len(fields) == 2 && fields[0] == "module" {
					_ = file.Close()
					return fields[1]
				}
			}
			_ = file.Close()
			return ""
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return ""
		}
		directory = parent
	}
}

func (l *concreteLayout) shouldBreak(
	open, close, column, width int,
	comments []cst.Comment,
	source []byte,
	preserveEmptyLines bool,
) bool {
	if close <= open+1 {
		return false
	}
	start := l.tokens[open].Position().Offset
	end := l.tokens[close].Position().Offset + len(l.tokens[close].Src())
	for _, comment := range comments {
		if comment.Start >= start && comment.End <= end {
			return true
		}
	}
	if preserveEmptyLines {
		previousEnd := l.tokens[open].Position().Offset + len(l.tokens[open].Src())
		for index := open + 1; index <= close; index++ {
			current := l.tokens[index]
			if countNewlines(source, previousEnd, current.Position().Offset) > 1 {
				return true
			}
			previousEnd = current.Position().Offset + len(current.Src())
		}
	}
	length := 0
	previous := token.ILLEGAL
	for index := open + 1; index <= close; index++ {
		current := l.tokens[index]
		if current.Ch() == token.SEMICOLON {
			continue
		}
		if l.softOpen[index] == 0 &&
			concreteNeedsSpace(previous, current.Ch(), l.spacedOps[index] || l.spaceBefore[index]) {
			length++
		}
		length += utf8.RuneCountInString(current.Src())
		if current.Ch() == token.COMMA {
			length++
		}
		previous = current.Ch()
	}
	return column+length > width
}

func (l *concreteLayout) renderCommentsBefore(
	writer *concreteWriter,
	comments []cst.Comment,
	commentIndex *int,
	source []byte,
	sourceEnd *int,
	beforeOffset int,
) {
	previousBuildConstraint := false
	for *commentIndex < len(comments) && comments[*commentIndex].Start < beforeOffset {
		comment := comments[*commentIndex]
		inline := *sourceEnd > 0 && countNewlines(source, *sourceEnd, comment.Start) == 0
		if inline {
			writer.pendingNewlines = 0
			writer.forceSpace = true
		} else if writer.output.Len() != 0 {
			writer.requestNewlines(1)
		}
		writer.preserveEmptyLines(source, *sourceEnd, comment.Start, previousBuildConstraint)
		writer.write(comment.Text, -1)
		writer.requestNewlines(1)
		*sourceEnd = comment.End
		previousBuildConstraint = isBuildConstraint(comment.Text)
		*commentIndex++
	}
	writer.preserveEmptyLines(source, *sourceEnd, beforeOffset, previousBuildConstraint)
}

func countNewlines(source []byte, start, end int) int {
	start = max(0, min(start, len(source)))
	end = max(start, min(end, len(source)))
	count := 0
	for _, current := range source[start:end] {
		if current == '\n' {
			count++
		}
	}
	return count
}

func isBuildConstraint(comment string) bool {
	return strings.HasPrefix(comment, "//go:build") || strings.HasPrefix(comment, "// +build")
}

func (w *concreteWriter) beforeToken(previous, current token.Token, force bool) {
	if w.lineStart || w.pendingNewlines != 0 {
		return
	}
	if force || w.forceSpace || concreteNeedsSpace(previous, current, false) {
		w.output.WriteByte(' ')
		w.column++
	}
	w.forceSpace = false
}

func concreteNeedsSpace(previous, current token.Token, spaced bool) bool {
	if previous == token.ILLEGAL || spaced {
		return spaced
	}
	if previous == token.LPAREN || previous == token.LBRACK || previous == token.PERIOD ||
		current == token.RPAREN || current == token.RBRACK || current == token.COMMA ||
		current == token.PERIOD || current == token.COLON || current == token.LPAREN ||
		current == token.LBRACK {
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
	return kind == token.IDENT || kind.IsKeyword() ||
		kind == token.INT || kind == token.FLOAT || kind == token.IMAG ||
		kind == token.CHAR || kind == token.STRING
}

func (w *concreteWriter) write(value string, indentOverride int) {
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
		w.column = indent * w.indentWidth
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

func (w *concreteWriter) requestNewlines(count int) {
	w.forceSpace = false
	count = min(count, w.maxEmptyLines+1)
	if count > w.pendingNewlines {
		w.pendingNewlines = count
	}
}

func (w *concreteWriter) requestRequiredNewlines(count int) {
	w.forceSpace = false
	if count > w.pendingNewlines {
		w.pendingNewlines = count
	}
}

func (w *concreteWriter) preserveEmptyLines(
	source []byte,
	start, end int,
	buildConstraint bool,
) {
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

func (w *concreteWriter) flushNewlines() {
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
