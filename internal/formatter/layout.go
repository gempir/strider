package formatter

import (
	"go/token"
	"unicode/utf8"

	"github.com/gempir/strider/internal/cst"
)

type layout struct {
	tree        *cst.Tree
	tokens      []cst.Token
	indices     map[cst.Token]int
	tokenLayout []tokenLayout
	importStart int
	importEnd   int
	imports     []importEntry
	module      string
}

type tokenLayout struct {
	hardOpen     int
	softOpen     int
	commaClose   int
	hardClose    bool
	softClose    bool
	forceBreak   bool
	softSemi     bool
	topSemi      bool
	spacedOp     bool
	spaceBefore  bool
	unaryOp      bool
	channelArrow bool
	spacedAfter  bool
	labelColon   bool
	caseToken    bool
	caseColon    bool
}

type delimiterStyle uint8

const (
	hardDelimiter delimiterStyle = iota
	softDelimiter
)

type tokenProperty uint8

const (
	propertyForceBreak tokenProperty = iota
	propertySoftSemi
	propertyTopSemi
	propertySpacedOp
	propertySpaceBefore
	propertyUnaryOp
	propertyChannelArrow
	propertySpacedAfter
	propertyLabelColon
	propertyCaseToken
	propertyCaseColon
)

func newLayout(tree *cst.Tree, module string) *layout {
	tokens := tree.Tokens()
	tokenCount := len(tokens)
	layout := &layout{
		tree:        tree,
		tokens:      tokens,
		indices:     make(map[cst.Token]int, tokenCount),
		tokenLayout: make([]tokenLayout, tokenCount),
		importStart: -1,
		importEnd:   -1,
		module:      module,
	}
	for index, current := range tokens {
		layout.indices[current] = index
		switch current.Ch() {
		case token.ASSIGN, token.DEFINE, token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN, token.QUO_ASSIGN, token.REM_ASSIGN, token.AND_ASSIGN, token.OR_ASSIGN, token.XOR_ASSIGN, token.SHL_ASSIGN, token.SHR_ASSIGN, token.AND_NOT_ASSIGN:
			layout.tokenLayout[index].spacedOp = true
		}
	}
	layout.indexTree()
	layout.indexImports()
	return layout
}

func (l *layout) indexTree() {
	cst.Walk(
		l.tree.Root(),
		func(node cst.Node) bool {
			kind := cst.Kind(node)
			switch kind {
			case "Block", "StructType", "InterfaceType", "ExprSwitchStmt", "TypeSwitchStmt", "SelectStmt":
				l.markDelimited(node, token.LBRACE, token.RBRACE, hardDelimiter)
			case "ConstDecl", "TypeDecl", "VarDecl":
				l.markDelimited(node, token.LPAREN, token.RPAREN, hardDelimiter)
			case "Parameters", "LiteralValue", "TypeParameters", "TypeArgs":
				l.markAnyDelimited(node, softDelimiter)
			default:
				if cst.IsArguments(node) {
					l.markDelimited(node, token.LPAREN, token.RPAREN, softDelimiter)
				}
			}
			if kind == "LiteralValue" {
				tokens := cst.NodeTokens(node)
				if len(tokens) != 0 {
					l.markToken(tokens[0], propertyForceBreak)
				}
			}
			switch kind {
			case "ForClause", "IfElseStmt", "IfStmt", "ExprSwitchStmt", "TypeSwitchStmt":
				l.markTokens(node, token.SEMICOLON, propertySoftSemi)
			case "SourceFile", "ImportDeclList", "TopLevelDeclList":
				l.markTokens(node, token.SEMICOLON, propertyTopSemi)
			}
			switch current := node.(type) {
			case *cst.BinaryExpression:
				l.markToken(current.Op, propertySpacedOp)
			case *cst.Assignment:
				l.markToken(current.Op, propertySpacedOp)
			case *cst.ShortVarDecl:
				l.markToken(current.DEFINE, propertySpacedOp)
			case *cst.UnaryExpr:
				l.markToken(current.Op, propertyUnaryOp)
			case *cst.ParameterDecl:
				if current.IdentifierList != nil {
					l.markFirstToken(current.TypeNode, propertySpaceBefore)
				}
			case *cst.VarSpec:
				if current.TypeNode != nil {
					l.markFirstToken(current.TypeNode, propertySpaceBefore)
				}
			case *cst.VarSpec2:
				if current.TypeNode != nil {
					l.markFirstToken(current.TypeNode, propertySpaceBefore)
				}
			case *cst.FieldDecl:
				if current.IdentifierList != nil && current.TypeNode != nil {
					l.markFirstToken(current.TypeNode, propertySpaceBefore)
				}
			case *cst.Result:
				if current.Parameters != nil {
					l.markFirstToken(current.Parameters, propertySpaceBefore)
				} else if current.TypeNode != nil {
					l.markFirstToken(current.TypeNode, propertySpaceBefore)
				}
			case *cst.MethodDecl:
				if current.Receiver != nil {
					l.markFirstToken(current.Receiver, propertySpaceBefore)
				}
			case *cst.TypeDef:
				l.markFirstToken(current.TypeNode, propertySpaceBefore)
			case *cst.TypeParamDecl:
				l.markFirstToken(current.TypeConstraint, propertySpaceBefore)
			case *cst.TypeElemList:
				l.markToken(current.OR, propertySpacedOp)
			}
			if kind == "SendStmt" || kind == "RangeClause" || kind == "TypeSwitchGuard" {
				for _, current := range cst.NodeTokens(node) {
					switch current.Ch() {
					case token.ARROW, token.ASSIGN, token.DEFINE:
						l.markToken(current, propertySpacedOp)
					}
				}
			}
			if kind == "RecvStmt" {
				for _, current := range cst.NodeTokens(node) {
					if current.Ch() == token.ASSIGN || current.Ch() == token.DEFINE {
						l.markToken(current, propertySpacedOp)
					}
				}
			}
			if kind == "ChannelType" {
				l.markTokens(node, token.ARROW, propertyChannelArrow)
			}
			if kind == "KeyedElement" {
				l.markTokens(node, token.COLON, propertySpacedAfter)
			}
			if kind == "LabeledStmt" {
				l.markTokens(node, token.COLON, propertyLabelColon)
			}
			if kind == "ExprCaseClauseList" || kind == "TypeCaseClause" || kind == "CommClause" {
				for _, current := range cst.NodeTokens(node) {
					switch current.Ch() {
					case token.CASE, token.DEFAULT:
						l.markToken(current, propertyCaseToken)
					case token.COLON:
						l.markToken(current, propertyCaseColon)
					}
				}
			}
			return true
		},
	)
}

func (l *layout) markAnyDelimited(node cst.Node, style delimiterStyle) {
	tokens := cst.NodeTokens(node)
	if len(tokens) < 2 {
		return
	}
	open, close := tokens[0].Ch(), tokens[len(tokens)-1].Ch()
	if (open == token.LPAREN && close == token.RPAREN) || (open == token.LBRACK && close == token.RBRACK) || (open == token.LBRACE && close == token.RBRACE) {
		l.markDelimited(node, open, close, style)
	}
}

func (l *layout) markDelimited(node cst.Node, openKind, closeKind token.Token, style delimiterStyle) {
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
		switch style {
		case hardDelimiter:
			l.tokenLayout[open].hardOpen = close
			l.tokenLayout[close].hardClose = true
		case softDelimiter:
			l.tokenLayout[open].softOpen = close
			l.tokenLayout[close].softClose = true
			l.markGroupCommas(node, close)
		}
	}
}

func (l *layout) markGroupCommas(node cst.Node, close int) {
	root := node
	cst.Walk(
		node,
		func(current cst.Node) bool {
			if current != root && delimitedNode(current) {
				return false
			}
			currentToken, ok := current.(cst.Token)
			if ok && currentToken.Ch() == token.COMMA {
				if index, found := l.tokenIndex(currentToken); found {
					l.tokenLayout[index].commaClose = close
				}
			}
			return true
		},
	)
}

func delimitedNode(node cst.Node) bool {
	if cst.IsArguments(node) {
		return true
	}
	switch cst.Kind(node) {
	case "Block", "StructType", "InterfaceType", "ExprSwitchStmt", "TypeSwitchStmt", "SelectStmt", "ConstDecl", "TypeDecl", "VarDecl", "Parameters", "LiteralValue", "TypeParameters", "TypeArgs":
		return true
	default:
		return false
	}
}

func (l *layout) markTokens(node cst.Node, wanted token.Token, property tokenProperty) {
	for _, child := range cst.Children(node) {
		current, ok := child.(cst.Token)
		if !ok {
			continue
		}
		if current.Ch() == wanted {
			l.markToken(current, property)
		}
	}
}

func (l *layout) markToken(current cst.Token, property tokenProperty) {
	if index, ok := l.tokenIndex(current); ok {
		currentLayout := &l.tokenLayout[index]
		switch property {
		case propertyForceBreak:
			currentLayout.forceBreak = true
		case propertySoftSemi:
			currentLayout.softSemi = true
		case propertyTopSemi:
			currentLayout.topSemi = true
		case propertySpacedOp:
			currentLayout.spacedOp = true
		case propertySpaceBefore:
			currentLayout.spaceBefore = true
		case propertyUnaryOp:
			currentLayout.unaryOp = true
		case propertyChannelArrow:
			currentLayout.channelArrow = true
		case propertySpacedAfter:
			currentLayout.spacedAfter = true
		case propertyLabelColon:
			currentLayout.labelColon = true
		case propertyCaseToken:
			currentLayout.caseToken = true
		case propertyCaseColon:
			currentLayout.caseColon = true
		}
	}
}

func (l *layout) markFirstToken(node cst.Node, property tokenProperty) {
	if node == nil {
		return
	}
	tokens := cst.NodeTokens(node)
	if len(tokens) != 0 {
		l.markToken(tokens[0], property)
	}
}

func (l *layout) tokenIndex(current cst.Token) (int, bool) {
	if !current.IsValid() {
		return 0, false
	}
	index, ok := l.indices[current]
	return index, ok
}

func (l *layout) shouldBreak(open, close, column, width int, comments []cst.Comment, source []byte, preserveEmptyLines bool) bool {
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
		currentLayout := l.tokenLayout[index]
		if currentLayout.softOpen == 0 && needsSpace(previous, current.Ch(), currentLayout.spacedOp || currentLayout.spaceBefore) {
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
