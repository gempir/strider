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
	"sort"
	"strings"
	"sync"

	gc "modernc.org/gc/v3"
)

var kindCache sync.Map

var childFieldsCache sync.Map

// Node is a production or token in the concrete syntax tree.
type Node = gc.Node

// Token is a lossless lexical token. Sep returns the comments and whitespace
// immediately before the token, and Src returns the token spelling.
type Token = gc.Token

// Grammar node aliases keep consumers coupled to Strider's CST vocabulary
// rather than to the parser implementation package.
type (
	AliasDecl         = gc.AliasDeclNode
	Assignment        = gc.AssignmentNode
	Arguments         = gc.ArgumentsNode
	Arguments1        = gc.Arguments1Node
	Arguments2        = gc.Arguments2Node
	Arguments3        = gc.Arguments3Node
	BasicLit          = gc.BasicLitNode
	BinaryExpression  = gc.BinaryExpressionNode
	Block             = gc.BlockNode
	BreakStmt         = gc.BreakStmtNode
	CommCase          = gc.CommCaseNode
	CommClause        = gc.CommClauseNode
	CommClauseList    = gc.CommClauseListNode
	ConstSpec         = gc.ConstSpecNode
	ConstSpec2        = gc.ConstSpec2Node
	DeferStmt         = gc.DeferStmtNode
	ExprSwitchCase    = gc.ExprSwitchCaseNode
	ExprSwitchCase2   = gc.ExprSwitchCase2Node
	ExprCaseClause    = gc.ExprCaseClauseListNode
	ExprSwitchStmt    = gc.ExprSwitchStmtNode
	ExpressionList    = gc.ExpressionListNode
	FallthroughStmt   = gc.FallthroughStmtNode
	FieldDecl         = gc.FieldDeclNode
	ForStmt           = gc.ForStmtNode
	FunctionBody      = gc.FunctionBodyNode
	FunctionDecl      = gc.FunctionDeclNode
	FunctionLit       = gc.FunctionLitNode
	IdentifierList    = gc.IdentifierListNode
	IfElseStmt        = gc.IfElseStmtNode
	IfStmt            = gc.IfStmtNode
	ImportDecl        = gc.ImportDeclNode
	ImportSpec        = gc.ImportSpecNode
	IncDecStmt        = gc.IncDecStmtNode
	InterfaceType     = gc.InterfaceTypeNode
	MethodDecl        = gc.MethodDeclNode
	ParameterDecl     = gc.ParameterDeclNode
	ParameterDeclList = gc.ParameterDeclListNode
	Parameters        = gc.ParametersNode
	ParenthesizedExpr = gc.ParenthesizedExpressionNode
	PrimaryExpr       = gc.PrimaryExprNode
	RangeClause       = gc.RangeClauseNode
	ReturnStmt        = gc.ReturnStmtNode
	Result            = gc.ResultNode
	SelectStmt        = gc.SelectStmtNode
	Selector          = gc.SelectorNode
	ShortVarDecl      = gc.ShortVarDeclNode
	Signature         = gc.SignatureNode
	StatementList     = gc.StatementListNode
	StructType        = gc.StructTypeNode
	Tag               = gc.TagNode
	TypeDef           = gc.TypeDefNode
	TypeElemList      = gc.TypeElemListNode
	TypeParamDecl     = gc.TypeParamDeclNode
	TypeSwitchCase    = gc.TypeSwitchCaseNode
	TypeSwitchStmt    = gc.TypeSwitchStmtNode
	TypeArgs          = gc.TypeArgsNode
	TypeAssertion     = gc.TypeAssertionNode
	TypeCaseClause    = gc.TypeCaseClauseNode
	TypeParameters    = gc.TypeParametersNode
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

	tokensOnce   sync.Once
	tokens       []Token
	commentsOnce sync.Once
	comments     []Comment
	linesOnce    sync.Once
	lines        []int
}

// Comment is a concrete source comment and its exact byte range.
type Comment struct {
	Text   string
	Start  int
	End    int
	Line   int
	Column int
}

type childField struct {
	index int
	slice bool
}

type bounds struct {
	first       Token
	last        Token
	firstOffset int
	lastOffset  int
	found       bool
}

type tokenWalkItem struct {
	node    Node
	token   Token
	isToken bool
}

// Parse parses one complete Go source file into a CST.
func Parse(filename string, source []byte) (*Tree, error) {
	root, err := gc.ParseFile(filename, source)
	if err != nil {
		return nil, err
	}
	// modernc's parser takes ownership of source and its tokens retain offsets
	// into that buffer. Keeping the same immutable slice avoids a redundant
	// full-file copy while preserving the parser's existing ownership contract.
	return &Tree{
		filename: filename,
		root:     root,
		source:   source,
	}, nil
}

// Root returns the source-file production. The EOF token is available through
// Tokens and is kept separately by the underlying parser.
func (t *Tree) Root() Node {
	if t == nil || t.root == nil {
		return nil
	}
	return t.root.SourceFile
}

// Bytes returns the original source without copying it. The returned slice is
// owned by the tree and must be treated as read-only.
func (t *Tree) Bytes() []byte {
	if t == nil {
		return nil
	}
	return t.source
}

// Comments returns all comments in source order without grouping or rewriting
// their original spelling. The returned slice is owned by the tree and must be
// treated as read-only.
func (t *Tree) Comments() []Comment {
	if t == nil {
		return nil
	}
	t.commentsOnce.Do(
		func() {
			fset := token.NewFileSet()
			file := fset.AddFile(t.filename, -1, len(t.source))
			var lexer scanner.Scanner
			lexer.Init(file, t.source, nil, scanner.ScanComments)
			for {
				position,
					kind,
					literal := lexer.Scan()
				if kind == token.EOF {
					break
				}
				if kind != token.COMMENT {
					continue
				}
				start := file.Offset(position)
				location := file.Position(position)
				t.comments = append(t.comments, Comment{
					Text:   literal,
					Start:  start,
					End:    start + len(literal),
					Line:   location.Line,
					Column: location.Column,
				})
			}
		},
	)
	return t.comments
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
	first, last, ok := nodeTokenBounds(node, false)
	if !ok {
		return ""
	}
	for current := first; current.IsValid(); current = current.Next() {
		if concreteToken(current) {
			result.WriteString(current.Src())
		}
		if current == last {
			break
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
	if kind, ok := generatedKind(node); ok {
		return kind
	}
	valueType := reflect.TypeOf(node)
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	if cached, ok := kindCache.Load(valueType); ok {
		return cached.(string)
	}
	kind := strings.TrimSuffix(valueType.Name(), "Node")
	kindCache.Store(valueType, kind)
	return kind
}

// Range returns the byte range occupied by a node's syntax, excluding leading
// trivia. End is exclusive. Empty implicit tokens do not extend the range.
func Range(node Node) (start, end int) {
	first, last, ok := nodeTokenBounds(node, true)
	if ok {
		start = first.Position().Offset
		end = last.Position().Offset + len(last.Src())
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
	if offset < 0 {
		offset = 0
	}
	if offset > len(t.source) {
		offset = len(t.source)
	}
	tokens := t.Tokens()
	index := sort.Search(len(tokens), func(index int) bool {
		return tokens[index].Position().Offset >= offset
	})
	if index < len(tokens) {
		position := tokens[index].Position()
		if position.Offset == offset {
			return position
		}
	}
	t.linesOnce.Do(func() {
		t.lines = make([]int, 1, len(t.source)/40+1)
		for index, current := range t.source {
			if current == '\n' {
				t.lines = append(t.lines, index+1)
			}
		}
	})
	lineIndex := sort.Search(len(t.lines), func(index int) bool {
		return t.lines[index] > offset
	}) - 1
	if lineIndex < 0 {
		lineIndex = 0
	}
	return token.Position{
		Filename: t.filename,
		Offset:   offset,
		Line:     lineIndex + 1,
		Column:   offset - t.lines[lineIndex] + 1,
	}
}

// Tokens returns every token in source order, including the EOF token. Trivia
// is stored in the following token, so the EOF token retains trailing trivia.
// The returned slice is owned by the tree and must be treated as read-only.
func (t *Tree) Tokens() []Token {
	if t == nil || t.root == nil || t.root.SourceFile == nil || t.root.SourceFile.PackageClause == nil {
		return nil
	}
	t.tokensOnce.Do(
		func() {
			current := t.root.SourceFile.PackageClause.PACKAGE
			for current.IsValid() {
				t.tokens = append(t.tokens, current)
				if current.Ch() == token.EOF {
					break
				}
				current = current.Next()
			}
		},
	)
	return t.tokens
}

// NodeTokens returns all tokens belonging to node in source order.
func NodeTokens(node Node) []Token {
	first, last, ok := nodeTokenBounds(node, false)
	if !ok {
		return nil
	}
	result := make([]Token, 0, 8)
	for current := first; current.IsValid(); current = current.Next() {
		result = append(result, current)
		if current == last {
			break
		}
	}
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
	if generated, ok := appendGeneratedChildren(result, node, false); ok {
		return generated
	}
	collectChildren(indirect(reflect.ValueOf(node)), &result)
	return result
}

// Walk visits node and its concrete children in structural grammar order.
// Returning false skips the current node's descendants.
func Walk(node Node, visit func(Node) bool) {
	if isNilNode(node) || visit == nil {
		return
	}
	var inline [32]Node
	stack := inline[:1]
	stack[0] = node
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if visit(current) {
			stack = appendChildrenReverse(stack, current)
		}
	}
}

// WalkWithAncestors visits a tree in structural grammar order and supplies the
// current node's ancestors from the root down. The ancestor slice is reused
// and must not be retained by the visitor.
func WalkWithAncestors(node Node, visit func(Node, []Node) bool) {
	if isNilNode(node) || visit == nil {
		return
	}
	type item struct {
		node Node
		exit bool
	}
	var inlineStack [32]item
	stack := inlineStack[:1]
	stack[0].node = node
	var inlineAncestors [16]Node
	ancestors := inlineAncestors[:0]
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if current.exit {
			ancestors = ancestors[:len(ancestors)-1]
			continue
		}
		if !visit(current.node, ancestors) {
			continue
		}
		stack = append(stack, item{
			exit: true,
		})
		ancestors = append(ancestors, current.node)
		stack = appendChildItemsReverse(stack, current.node)
	}
}

// WalkProductionsWithAncestors visits only grammar productions in structural
// order and supplies their production ancestors from the root down. Token
// leaves are skipped, avoiding their conversion to Node interface values. The
// ancestor slice is reused and must not be retained by the visitor.
func WalkProductionsWithAncestors(node Node, visit func(Node, []Node) bool) {
	if isNilNode(node) || tokenNode(node) || visit == nil {
		return
	}
	type item struct {
		node Node
		exit bool
	}
	var inlineStack [32]item
	stack := inlineStack[:1]
	stack[0].node = node
	var inlineAncestors [16]Node
	ancestors := inlineAncestors[:0]
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if current.exit {
			ancestors = ancestors[:len(ancestors)-1]
			continue
		}
		if !visit(current.node, ancestors) {
			continue
		}
		stack = append(stack, item{
			exit: true,
		})
		ancestors = append(ancestors, current.node)
		stack = appendProductionChildItemsReverse(stack, current.node)
	}
}

func collectChildren(value reflect.Value, result *[]Node) {
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return
	}
	for _, plan := range childFields(value.Type()) {
		field := value.Field(plan.index)
		if child, ok := nodeValue(field); ok {
			*result = append(*result, child)
			continue
		}
		if plan.slice {
			for item := 0; item < field.Len(); item++ {
				if child, ok := nodeValue(field.Index(item)); ok {
					*result = append(*result, child)
				}
			}
		}
	}
}

func childFields(valueType reflect.Type) []childField {
	if cached, ok := childFieldsCache.Load(valueType); ok {
		return cached.([]childField)
	}
	fields := make([]childField, 0, valueType.NumField())
	for index := 0; index < valueType.NumField(); index++ {
		field := valueType.Field(index)
		if !field.IsExported() {
			continue
		}
		fields = append(fields, childField{
			index: index,
			slice: field.Type.Kind() == reflect.Slice,
		})
	}
	childFieldsCache.Store(valueType, fields)
	return fields
}

func appendChildrenReverse(stack []Node, node Node) []Node {
	if generated, ok := appendGeneratedChildren(stack, node, true); ok {
		return generated
	}
	if _, ok := node.(Token); ok {
		return stack
	}
	value := indirect(reflect.ValueOf(node))
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return stack
	}
	fields := childFields(value.Type())
	for fieldIndex := len(fields) - 1; fieldIndex >= 0; fieldIndex-- {
		plan := fields[fieldIndex]
		field := value.Field(plan.index)
		if child, ok := nodeValue(field); ok {
			stack = append(stack, child)
			continue
		}
		if plan.slice {
			for item := field.Len() - 1; item >= 0; item-- {
				if child, ok := nodeValue(field.Index(item)); ok {
					stack = append(stack, child)
				}
			}
		}
	}
	return stack
}

func appendChildItemsReverse[T ~struct {
	node Node
	exit bool
}](stack []T, node Node) []T {
	if generated, ok := appendGeneratedChildItemsReverse(stack, node, true); ok {
		return generated
	}
	if _, ok := node.(Token); ok {
		return stack
	}
	value := indirect(reflect.ValueOf(node))
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return stack
	}
	fields := childFields(value.Type())
	for fieldIndex := len(fields) - 1; fieldIndex >= 0; fieldIndex-- {
		plan := fields[fieldIndex]
		field := value.Field(plan.index)
		if child, ok := nodeValue(field); ok {
			stack = append(stack, T{
				node: child,
			})
			continue
		}
		if plan.slice {
			for item := field.Len() - 1; item >= 0; item-- {
				if child, ok := nodeValue(field.Index(item)); ok {
					stack = append(stack, T{
						node: child,
					})
				}
			}
		}
	}
	return stack
}

func appendProductionChildItemsReverse[T ~struct {
	node Node
	exit bool
}](stack []T, node Node) []T {
	if generated, ok := appendGeneratedChildItemsReverse(stack, node, false); ok {
		return generated
	}
	if tokenNode(node) {
		return stack
	}
	value := indirect(reflect.ValueOf(node))
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return stack
	}
	fields := childFields(value.Type())
	for fieldIndex := len(fields) - 1; fieldIndex >= 0; fieldIndex-- {
		plan := fields[fieldIndex]
		field := value.Field(plan.index)
		if child, ok := nodeValue(field); ok {
			if !tokenNode(child) {
				stack = append(stack, T{
					node: child,
				})
			}
			continue
		}
		if plan.slice {
			for item := field.Len() - 1; item >= 0; item-- {
				if child, ok := nodeValue(field.Index(item)); ok && !tokenNode(child) {
					stack = append(stack, T{
						node: child,
					})
				}
			}
		}
	}
	return stack
}

func nodeTokenBounds(node Node, concreteOnly bool) (Token, Token, bool) {
	if isNilNode(node) {
		return Token{}, Token{}, false
	}
	result := bounds{}
	var inline [32]tokenWalkItem
	stack := inline[:1]
	stack[0].node = node
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if current.isToken {
			includeTokenBounds(current.token, concreteOnly, &result)
			continue
		}
		if currentToken, ok := current.node.(Token); ok {
			includeTokenBounds(currentToken, concreteOnly, &result)
			continue
		}
		if generated, ok := appendGeneratedTokenItemsReverse(stack, current.node); ok {
			stack = generated
			continue
		}
		collectTokenBounds(reflect.ValueOf(current.node), concreteOnly, &result)
	}
	return result.first, result.last, result.found
}

func includeTokenBounds(current Token, concreteOnly bool, result *bounds) {
	if !current.IsValid() || (concreteOnly && !concreteToken(current)) {
		return
	}
	offset := current.Position().Offset
	if !result.found || offset < result.firstOffset {
		result.first = current
		result.firstOffset = offset
	}
	if !result.found || offset >= result.lastOffset {
		result.last = current
		result.lastOffset = offset
	}
	result.found = true
}

func collectTokenBounds(value reflect.Value, concreteOnly bool, result *bounds) {
	if !value.IsValid() {
		return
	}
	if current, ok := tokenValue(value); ok {
		includeTokenBounds(current, concreteOnly, result)
		return
	}
	value = indirect(value)
	if !value.IsValid() {
		return
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		collectTokenBounds(value.Elem(), concreteOnly, result)
	case reflect.Struct:
		for _, field := range childFields(value.Type()) {
			collectTokenBounds(value.Field(field.index), concreteOnly, result)
		}
	case reflect.Slice:
		for index := 0; index < value.Len(); index++ {
			collectTokenBounds(value.Index(index), concreteOnly, result)
		}
	}
}

func nodeValue(value reflect.Value) (Node, bool) {
	if !value.IsValid() || (nilable(value.Kind()) && value.IsNil()) || !value.CanInterface() {
		return nil, false
	}
	if current, ok := value.Interface().(Token); ok && !current.IsValid() {
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
	if result, ok := generatedNodeNil(node); ok {
		return result
	}
	value := reflect.ValueOf(node)
	return nilable(value.Kind()) && value.IsNil()
}

func nodePresent(node Node) bool {
	if present, ok := generatedNodePresent(node); ok {
		return present
	}
	if node == nil {
		return false
	}
	if current, ok := node.(Token); ok && !current.IsValid() {
		return false
	}
	return !isNilNode(node)
}

func tokenNode(node Node) bool {
	switch node.(type) {
	case Token, *Token:
		return true
	default:
		return false
	}
}

func nilable(kind reflect.Kind) bool {
	return kind == reflect.Chan || kind == reflect.Func || kind == reflect.Interface || kind == reflect.Map || kind == reflect.Pointer || kind == reflect.Slice
}
