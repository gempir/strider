// Package cst provides Strider's lossless concrete syntax tree for Go source.
//
// Formatting and file-local linting use this package so their view of the
// program includes every token and separator. Package analysis deliberately
// uses go/ast instead because type checking and SSA operate on that tree.
package cst

import (
	"go/scanner"
	"go/token"
	"reflect"
	"strings"

	gc "modernc.org/gc/v3"
)

// Node is a production or token in the concrete syntax tree.
type Node = gc.Node

// Token is a lossless lexical token. Sep returns the comments and whitespace
// immediately before the token, and Src returns the token spelling.
type Token = gc.Token

// Grammar node aliases keep consumers coupled to Strider's CST vocabulary
// rather than to the parser implementation package.
type (
	Assignment        = gc.AssignmentNode
	BasicLit          = gc.BasicLitNode
	BinaryExpression  = gc.BinaryExpressionNode
	Block             = gc.BlockNode
	CommCase          = gc.CommCaseNode
	DeferStmt         = gc.DeferStmtNode
	ExprSwitchCase    = gc.ExprSwitchCaseNode
	ExprSwitchCase2   = gc.ExprSwitchCase2Node
	ForStmt           = gc.ForStmtNode
	FunctionDecl      = gc.FunctionDeclNode
	FunctionLit       = gc.FunctionLitNode
	IdentifierList    = gc.IdentifierListNode
	IfElseStmt        = gc.IfElseStmtNode
	InterfaceType     = gc.InterfaceTypeNode
	MethodDecl        = gc.MethodDeclNode
	ParameterDecl     = gc.ParameterDeclNode
	ParameterDeclList = gc.ParameterDeclListNode
	Parameters        = gc.ParametersNode
	ParenthesizedExpr = gc.ParenthesizedExpressionNode
	ReturnStmt        = gc.ReturnStmtNode
	ShortVarDecl      = gc.ShortVarDeclNode
	StatementList     = gc.StatementListNode
	TypeSwitchCase    = gc.TypeSwitchCaseNode
	TypeSwitchStmt    = gc.TypeSwitchStmtNode
	UnaryExpr         = gc.UnaryExprNode
	VarDecl           = gc.VarDeclNode
	VarSpec           = gc.VarSpecNode
	VarSpec2          = gc.VarSpec2Node
)

// Tree owns one parsed source file and its lossless concrete representation.
type Tree struct {
	filename string
	root     *gc.AST
	source   []byte
}

// Comment is a concrete source comment and its exact byte range.
type Comment struct {
	Text   string
	Start  int
	End    int
	Line   int
	Column int
}

// Parse parses one complete Go source file into a CST.
func Parse(filename string, source []byte) (*Tree, error) {
	root, err := gc.ParseFile(filename, source)
	if err != nil {
		return nil, err
	}
	return &Tree{filename: filename, root: root, source: append([]byte(nil), source...)}, nil
}

// Root returns the source-file production. The EOF token is available through
// Tokens and is kept separately by the underlying parser.
func (t *Tree) Root() Node {
	if t == nil || t.root == nil {
		return nil
	}
	return t.root.SourceFile
}

// Source returns an independent copy of the original bytes.
func (t *Tree) Source() []byte {
	if t == nil {
		return nil
	}
	return append([]byte(nil), t.source...)
}

// Comments returns all comments in source order without grouping or rewriting
// their original spelling.
func (t *Tree) Comments() []Comment {
	if t == nil {
		return nil
	}
	fset := token.NewFileSet()
	file := fset.AddFile(t.filename, -1, len(t.source))
	var lexer scanner.Scanner
	lexer.Init(file, t.source, nil, scanner.ScanComments)
	result := []Comment{}
	for {
		position, kind, literal := lexer.Scan()
		if kind == token.EOF {
			return result
		}
		if kind != token.COMMENT {
			continue
		}
		start := file.Offset(position)
		location := file.Position(position)
		result = append(result, Comment{
			Text:   literal,
			Start:  start,
			End:    start + len(literal),
			Line:   location.Line,
			Column: location.Column,
		})
	}
}

// Text reconstructs a node with all of its original whitespace and comments.
func Text(node Node) string {
	if isNilNode(node) {
		return ""
	}
	return node.Source(true)
}

// Spelling returns only the lexical spelling of a node, without comments or
// whitespace trivia.
func Spelling(node Node) string {
	var result strings.Builder
	for _, current := range NodeTokens(node) {
		if concreteToken(current) {
			result.WriteString(current.Src())
		}
	}
	return result.String()
}

// Kind returns a stable production name without the implementation's Node
// suffix. Tokens use the spelling from go/token, such as "func" or IDENT.
func Kind(node Node) string {
	if isNilNode(node) {
		return ""
	}
	if current, ok := node.(gc.Token); ok {
		return current.Ch().String()
	}
	valueType := reflect.TypeOf(node)
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	return strings.TrimSuffix(valueType.Name(), "Node")
}

// Range returns the byte range occupied by a node's syntax, excluding leading
// trivia. End is exclusive. Empty implicit tokens do not extend the range.
func Range(node Node) (start, end int) {
	tokens := NodeTokens(node)
	for _, current := range tokens {
		if !concreteToken(current) {
			continue
		}
		start = current.Position().Offset
		break
	}
	for index := len(tokens) - 1; index >= 0; index-- {
		current := tokens[index]
		if !concreteToken(current) {
			continue
		}
		end = current.Position().Offset + len(current.Src())
		break
	}
	return start, end
}

func concreteToken(current Token) bool {
	if current.Src() == "" {
		return false
	}
	return current.Ch() != token.SEMICOLON || current.Src() == ";"
}

// Position returns a go/token position for an offset in this tree. Offsets at
// EOF are supported.
func (t *Tree) Position(offset int) token.Position {
	if t == nil || t.root == nil {
		return token.Position{}
	}
	for _, current := range t.Tokens() {
		position := current.Position()
		if position.Offset >= offset {
			if position.Offset == offset {
				return position
			}
			break
		}
	}
	return positionAt(t.source, offset)
}

// Tokens returns every token in source order, including the EOF token. Trivia
// is stored in the following token, so the EOF token retains trailing trivia.
func (t *Tree) Tokens() []Token {
	if t == nil || t.root == nil || t.root.SourceFile == nil ||
		t.root.SourceFile.PackageClause == nil {
		return nil
	}
	current := t.root.SourceFile.PackageClause.PACKAGE
	result := []Token{}
	for current.IsValid() {
		result = append(result, current)
		if current.Ch() == token.EOF {
			break
		}
		current = current.Next()
	}
	return result
}

// NodeTokens returns all tokens belonging to node in source order.
func NodeTokens(node Node) []Token {
	if isNilNode(node) {
		return nil
	}
	result := []Token{}
	collectTokens(reflect.ValueOf(node), &result)
	return result
}

// Children returns the direct grammar children of node. Tokens are included
// as leaves, which makes the traversal concrete rather than abstract.
func Children(node Node) []Node {
	if isNilNode(node) {
		return nil
	}
	if _, ok := node.(gc.Token); ok {
		return nil
	}
	result := []Node{}
	collectChildren(indirect(reflect.ValueOf(node)), &result)
	return result
}

// Walk visits node and its concrete children in source order. Returning false
// skips the current node's descendants.
func Walk(node Node, visit func(Node) bool) {
	if isNilNode(node) || visit == nil || !visit(node) {
		return
	}
	for _, child := range Children(node) {
		Walk(child, visit)
	}
}

func collectChildren(value reflect.Value, result *[]Node) {
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return
	}
	valueType := value.Type()
	for index := 0; index < value.NumField(); index++ {
		fieldInfo := valueType.Field(index)
		if !fieldInfo.IsExported() {
			continue
		}
		field := value.Field(index)
		if child, ok := nodeValue(field); ok {
			*result = append(*result, child)
			continue
		}
		if field.Kind() == reflect.Slice {
			for item := 0; item < field.Len(); item++ {
				if child, ok := nodeValue(field.Index(item)); ok {
					*result = append(*result, child)
				}
			}
		}
	}
}

func collectTokens(value reflect.Value, result *[]Token) {
	if !value.IsValid() {
		return
	}
	if current, ok := tokenValue(value); ok {
		if current.IsValid() {
			*result = append(*result, current)
		}
		return
	}
	value = indirect(value)
	if !value.IsValid() {
		return
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		collectTokens(value.Elem(), result)
	case reflect.Struct:
		valueType := value.Type()
		for index := 0; index < value.NumField(); index++ {
			if valueType.Field(index).IsExported() {
				collectTokens(value.Field(index), result)
			}
		}
	case reflect.Slice:
		for index := 0; index < value.Len(); index++ {
			collectTokens(value.Index(index), result)
		}
	}
}

func nodeValue(value reflect.Value) (Node, bool) {
	if !value.IsValid() || (nilable(value.Kind()) && value.IsNil()) || !value.CanInterface() {
		return nil, false
	}
	node, ok := value.Interface().(Node)
	return node, ok && !isNilNode(node)
}

func tokenValue(value reflect.Value) (Token, bool) {
	if !value.IsValid() || !value.CanInterface() {
		return Token{}, false
	}
	current, ok := value.Interface().(Token)
	return current, ok
}

func indirect(value reflect.Value) reflect.Value {
	for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}

func isNilNode(node Node) bool {
	if node == nil {
		return true
	}
	value := reflect.ValueOf(node)
	return nilable(value.Kind()) && value.IsNil()
}

func nilable(kind reflect.Kind) bool {
	return kind == reflect.Chan || kind == reflect.Func || kind == reflect.Interface ||
		kind == reflect.Map || kind == reflect.Pointer || kind == reflect.Slice
}

func positionAt(source []byte, offset int) token.Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	line, column := 1, 1
	for _, current := range source[:offset] {
		if current == '\n' {
			line, column = line+1, 1
			continue
		}
		column++
	}
	return token.Position{Offset: offset, Line: line, Column: column}
}
