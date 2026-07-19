package rules

import (
	"go/token"
	"strconv"

	"github.com/gempir/strider/internal/cst"
)

type cstExecutionPlan struct {
	filename             bool
	comments             bool
	imports              bool
	repeatedLiterals     bool
	functions            bool
	functionTraversal    bool
	functionComplexity   bool
	functionCognitive    bool
	functionStatements   bool
	functionFinal        bool
	returns              bool
	defers               bool
	conditionals         bool
	packageVars          bool
	binaryExpressions    bool
	unaryExpressions     bool
	interfaces           bool
	incrementAssignments bool
	assignmentPolicies   bool
	identifiers          bool
	calls                bool
	structs              bool
	fields               bool
	stringLiterals       bool
	blocks               bool
	loops                bool
	controlNesting       bool
	switches             bool
	typeAssertions       bool
	varSpecs             bool
	constSpecs           bool
	typeDefinitions      bool
	breaks               bool
}

type cstFunctionFacts struct {
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

func compileCSTExecutionPlan(enabled map[string]bool) cstExecutionPlan {
	any := func(codes ...string) bool {
		for _, code := range codes {
			if enabled[code] {
				return true
			}
		}
		return false
	}
	identifiers := any("banned-characters", "confusing-naming", "import-shadowing", "redefines-builtin-id", "unexported-naming", "var-naming")
	functionComplexity := enabled["cyclomatic-complexity"]
	functionCognitive := enabled["cognitive-complexity"]
	functionStatements := enabled["function-length"]
	functionFinal := enabled["redundant-final-return"]
	return cstExecutionPlan{
		filename: any("filename-format", "package-directory-mismatch", "package-naming"),
		comments: any("bidirectional-control-character", "file-length-limit", "package-comments", "redundant-build-tag", "spaced-compiler-directive"),
		imports: any(
			"blank-imports",
			"dot-imports",
			"duplicated-imports",
			"import-alias-naming",
			"import-shadowing",
			"imports-blocklist",
			"redundant-import-alias",
			"invalid-struct-tag",
		),
		repeatedLiterals: enabled["add-constant"],
		functions: any(
			"cognitive-complexity",
			"confusing-results",
			"context-as-argument",
			"cyclomatic-complexity",
			"error-last-result",
			"exported-declaration-comment",
			"flag-parameter",
			"function-length",
			"function-result-limit",
			"get-function-return-value",
			"marshal-receiver",
			"max-parameters",
			"modifies-parameter",
			"modifies-value-receiver",
			"receiver-naming",
			"time-naming",
			"unexported-return",
			"redundant-final-return",
			"unused-parameter",
			"unused-receiver",
			"waitgroup-by-value",
		),
		functionTraversal:  functionComplexity || functionCognitive || functionStatements || functionFinal,
		functionComplexity: functionComplexity,
		functionCognitive:  functionCognitive,
		functionStatements: functionStatements,
		functionFinal:      functionFinal,
		returns:            enabled["no-naked-return"],
		defers:             any("deferred-recover-call", "discarded-deferred-result", "no-defer-in-loop"),
		conditionals: any(
			"early-return",
			"identical-branches",
			"identical-if-chain-branches",
			"identical-if-chain-conditions",
			"inefficient-map-lookup",
			"unnecessary-if",
		),
		packageVars:          enabled["no-package-var"],
		binaryExpressions:    any("boolean-literal-comparison", "constant-logical-expr", "modulo-one", "optimize-operands-order", "zero-integer-division"),
		unaryExpressions:     any("double-negation", "ineffective-pointer-copy"),
		interfaces:           enabled["use-any"],
		incrementAssignments: enabled["increment-decrement"],
		assignmentPolicies:   enabled["redundant-atomic-result-assignment"],
		identifiers:          identifiers,
		calls: any(
			"call-to-gc",
			"deep-exit",
			"error-strings",
			"prefer-fmt-errorf",
			"string-of-int",
			"time-date",
			"unnecessary-format",
			"use-errors-new",
			"use-fmt-print",
			"use-slices-sort",
		),
		structs:         enabled["nested-structs"],
		fields:          enabled["invalid-struct-tag"],
		stringLiterals:  enabled["insecure-url-scheme"],
		blocks:          any("empty-conditional-block", "redundant-error-return-check", "unreachable-code"),
		loops:           any("simplify-range", "range-value-address", "spinning-select-default"),
		controlNesting:  enabled["max-control-nesting"],
		switches:        any("enforce-switch-style", "identical-switch-branches", "identical-switch-conditions", "single-case-switch"),
		typeAssertions:  enabled["unchecked-type-assertion"],
		varSpecs:        any("error-naming", "exported-declaration-comment", "time-naming", "var-declaration"),
		constSpecs:      enabled["exported-declaration-comment"],
		typeDefinitions: any("exported-declaration-comment", "max-public-structs"),
		breaks:          enabled["redundant-switch-break"],
	}
}

func (a *cstAnalyzer) observe(node cst.Node, ancestors []cst.Node) {
	if a.plan.functions {
		var facts *cstFunctionFacts
		switch current := node.(type) {
		case *cst.FunctionDecl:
			if current.FunctionName != nil {
				facts = a.addFunctionFacts(current, current.FunctionName.IDENT, current.Signature, current.FunctionBody, nil)
			}
		case *cst.MethodDecl:
			facts = a.addFunctionFacts(current, current.MethodName, current.Signature, current.FunctionBody, current.Receiver)
		}
		if facts != nil && a.plan.functionTraversal {
			a.activeFunction = facts
			a.functionDepth = len(ancestors)
			a.functionBodyDepth = -1
		} else {
			a.observeFunctionNode(node, ancestors)
		}
	}
	if a.plan.repeatedLiterals {
		if literal, ok := node.(*cst.BasicLit); ok {
			a.observeRepeatedLiteral(literal, ancestors)
		}
	}
}

func (a *cstAnalyzer) addFunctionFacts(node cst.Node, name cst.Token, signature *cst.Signature, body cst.Node, receiver *cst.Parameters) *cstFunctionFacts {
	facts := &cstFunctionFacts{
		node:      node,
		name:      name,
		signature: signature,
		body:      body,
		receiver:  receiver,
	}
	if a.plan.functionFinal {
		facts.finalStatement = concreteDirectFinalStatement(body)
	}
	if a.plan.functionComplexity && body != nil {
		facts.complexity = 1
	}
	a.functions = append(a.functions, facts)
	return facts
}

func (a *cstAnalyzer) observeFunctionNode(node cst.Node, ancestors []cst.Node) {
	facts := a.activeFunction
	if facts == nil {
		return
	}
	if len(ancestors) <= a.functionDepth || ancestors[a.functionDepth] != facts.node {
		a.activeFunction = nil
		return
	}
	if node == facts.body {
		a.functionBodyDepth = len(ancestors)
	}
	if a.functionBodyDepth < 0 || (node != facts.body && (len(ancestors) <= a.functionBodyDepth || ancestors[a.functionBodyDepth] != facts.body)) {
		return
	}
	if a.plan.functionStatements {
		if list, ok := node.(*cst.StatementList); ok && list.Statement != nil {
			facts.statements++
		}
	}
	if a.plan.functionComplexity {
		switch current := node.(type) {
		case *cst.IfStmt, *cst.IfElseStmt, *cst.ForStmt, *cst.TypeSwitchStmt:
			facts.complexity++
		case *cst.CommCase:
			if current.CASE.IsValid() {
				facts.complexity++
			}
		case *cst.ExprSwitchCase:
			if current.CASE.IsValid() {
				facts.complexity++
			}
		case *cst.ExprSwitchCase2:
			if current.CASE.IsValid() {
				facts.complexity++
			}
		case *cst.TypeSwitchCase:
			if current.CASE.IsValid() {
				facts.complexity++
			}
		case *cst.BinaryExpression:
			if current.Op.Ch() == token.LAND || current.Op.Ch() == token.LOR {
				facts.complexity++
			}
		}
	}
	if !a.plan.functionCognitive {
		return
	}
	if concreteCognitiveControl(node) {
		nesting := 0
		for _, ancestor := range ancestors[a.functionBodyDepth+1:] {
			if concreteCognitiveControl(ancestor) {
				nesting++
			}
		}
		facts.cognitiveComplexity += 1 + nesting
		return
	}
	switch cst.Kind(node) {
	case "BreakStmt", "ContinueStmt", "GotoStmt", "FallthroughStmt":
		facts.cognitiveComplexity++
	}
}

func concreteCognitiveControl(node cst.Node) bool {
	switch node.(type) {
	case *cst.IfStmt, *cst.IfElseStmt, *cst.ForStmt, *cst.ExprSwitchStmt, *cst.TypeSwitchStmt, *cst.SelectStmt:
		return true
	default:
		return false
	}
}

func concreteDirectFinalStatement(body cst.Node) cst.Node {
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

func (a *cstAnalyzer) observeRepeatedLiteral(literal *cst.BasicLit, ancestors []cst.Node) {
	if literal.Ch() != token.STRING {
		return
	}
	for _, ancestor := range ancestors {
		switch cst.Kind(ancestor) {
		case "ConstDecl", "VarDecl", "TypeDecl":
			return
		}
	}
	value, err := strconv.Unquote(literal.Src())
	if err != nil {
		return
	}
	if value != "" {
		a.repeatedLiterals[literal.Src()] = append(a.repeatedLiterals[literal.Src()], literal)
	}
}
