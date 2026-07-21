package rules

import (
	"go/token"

	"github.com/gempir/strider/internal/cst"
)

type functionFacts struct {
	node                cst.Node
	name                cst.Token
	signature           *cst.Signature
	body                cst.Node
	receiver            *cst.Parameters
	complexity          int
	cognitiveComplexity int
	statements          int
	finalStatement      cst.Node
}

func (a *Pass) functionFacts(node cst.Node) *functionFacts {
	var facts functionFacts
	switch current := node.(type) {
	case *cst.FunctionDecl:
		if current.FunctionName == nil {
			return &facts
		}
		facts = functionFacts{
			node:      current,
			name:      current.FunctionName.IDENT,
			signature: current.Signature,
			body:      current.FunctionBody,
		}
	case *cst.MethodDecl:
		facts = functionFacts{
			node:      current,
			name:      current.MethodName,
			signature: current.Signature,
			body:      current.FunctionBody,
			receiver:  current.Receiver,
		}
	default:
		return &facts
	}
	facts.finalStatement = directFinalStatement(facts.body)
	if facts.body == nil {
		return &facts
	}
	facts.complexity = 1
	cst.WalkProductionsWithAncestors(
		facts.body,
		func(current cst.Node, ancestors []cst.Node) bool {
			if list, ok := current.(*cst.StatementList); ok && list.Statement != nil {
				facts.statements++
			}
			switch typed := current.(type) {
			case *cst.IfStmt, *cst.IfElseStmt, *cst.ForStmt, *cst.TypeSwitchStmt:
				facts.complexity++
			case *cst.CommCase:
				if typed.CASE.IsValid() {
					facts.complexity++
				}
			case *cst.ExprSwitchCase:
				if typed.CASE.IsValid() {
					facts.complexity++
				}
			case *cst.ExprSwitchCase2:
				if typed.CASE.IsValid() {
					facts.complexity++
				}
			case *cst.TypeSwitchCase:
				if typed.CASE.IsValid() {
					facts.complexity++
				}
			case *cst.BinaryExpression:
				if typed.Op.Ch() == token.LAND || typed.Op.Ch() == token.LOR {
					facts.complexity++
				}
			}
			if cognitiveControl(current) {
				nesting := 0
				for _, ancestor := range ancestors {
					if cognitiveControl(ancestor) {
						nesting++
					}
				}
				facts.cognitiveComplexity += 1 + nesting
				return true
			}
			switch cst.Kind(current) {
			case "BreakStmt", "ContinueStmt", "GotoStmt", "FallthroughStmt":
				facts.cognitiveComplexity++
			}
			return true
		},
	)
	return &facts
}

func cognitiveControl(node cst.Node) bool {
	switch node.(type) {
	case *cst.IfStmt, *cst.IfElseStmt, *cst.ForStmt, *cst.ExprSwitchStmt, *cst.TypeSwitchStmt, *cst.SelectStmt:
		return true
	default:
		return false
	}
}

func directFinalStatement(body cst.Node) cst.Node {
	var block *cst.Block
	switch current := body.(type) {
	case *cst.FunctionBody:
		if current != nil {
			block = current.Block
		}
	case *cst.Block:
		block = current
	}
	if block == nil {
		return nil
	}
	var final cst.Node
	for list := block.StatementList; list != nil; list = list.List {
		if list.Statement != nil {
			final = list.Statement
		}
	}
	return final
}
