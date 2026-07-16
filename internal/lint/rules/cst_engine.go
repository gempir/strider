package rules

import (
	"fmt"
	"go/token"

	"github.com/gempir/strider/internal/cst"
)

var cstRuleCodes = map[string]bool{
	"bidirectional-control-character": true,
	"banned-characters":               true,
	"blank-imports":                   true,
	"bool-literal-in-expr":            true,
	"comment-spacings":                true,
	"comments-density":                true,
	"cyclomatic-complexity":           true,
	"double-negation":                 true,
	"dot-imports":                     true,
	"duplicated-imports":              true,
	"enforce-map-style":               true,
	"enforce-repeated-arg-type-style": true,
	"enforce-slice-style":             true,
	"file-header":                     true,
	"file-length-limit":               true,
	"filename-format":                 true,
	"import-alias-naming":             true,
	"imports-blocklist":               true,
	"ineffective-pointer-copy":        true,
	"increment-decrement":             true,
	"line-length-limit":               true,
	"max-parameters":                  true,
	"modulo-one":                      true,
	"no-defer-in-loop":                true,
	"no-else-after-return":            true,
	"no-init":                         true,
	"no-naked-return":                 true,
	"no-package-var":                  true,
	"package-comments":                true,
	"package-directory-mismatch":      true,
	"package-naming":                  true,
	"redundant-build-tag":             true,
	"redundant-import-alias":          true,
	"spaced-compiler-directive":       true,
	"string-format":                   true,
	"use-any":                         true,
	"zero-integer-division":           true,
}

// UsesCST reports whether a rule has moved to the concrete-syntax pass.
func UsesCST(code string) bool { return cstRuleCodes[code] }

type cstAnalyzer struct {
	filename  string
	tree      *cst.Tree
	content   []byte
	enabled   map[string]bool
	reporter  func(Finding)
	ancestors []cst.Node
	current   cst.Node
}

// AnalyzeCST runs selected native CST rules over one lossless source tree.
func AnalyzeCST(input CSTInput) {
	enabled := make(map[string]bool, len(input.Rules))
	for _, rule := range input.Rules {
		if UsesCST(rule.Meta().Code) {
			enabled[rule.Meta().Code] = true
		}
	}
	if len(enabled) == 0 || input.Tree == nil {
		return
	}
	analyzer := &cstAnalyzer{
		filename: input.Filename,
		tree:     input.Tree,
		content:  input.Tree.Source(),
		enabled:  enabled,
		reporter: input.Report,
	}
	analyzer.checkFile()
	stack := []cst.Node{}
	var visit func(cst.Node)
	visit = func(node cst.Node) {
		analyzer.ancestors = stack
		analyzer.current = node
		analyzer.check(node)
		stack = append(stack, node)
		for _, child := range cst.Children(node) {
			visit(child)
		}
		stack = stack[:len(stack)-1]
	}
	visit(input.Tree.Root())
}

func (a *cstAnalyzer) check(node cst.Node) {
	switch current := node.(type) {
	case *cst.FunctionDecl:
		a.checkFunction(current)
	case *cst.MethodDecl:
		a.checkMethod(current)
	case *cst.ReturnStmt:
		a.checkNakedReturn(current)
	case *cst.DeferStmt:
		a.checkDefer(current)
	case *cst.IfElseStmt:
		a.checkElseAfterReturn(current)
	case *cst.VarDecl:
		a.checkPackageVar(current)
	case *cst.BinaryExpression:
		a.checkBinaryExpression(current)
	case *cst.UnaryExpr:
		a.checkUnaryExpression(current)
	case *cst.InterfaceType:
		a.checkInterfaceType(current)
	case *cst.Assignment:
		a.checkIncrementAssignment(current)
	case *cst.ShortVarDecl:
		a.checkIncrementShortDeclaration(current)
	}
}

func (a *cstAnalyzer) report(code string, node cst.Node, message string) {
	if !a.enabled[code] || node == nil || a.reporter == nil {
		return
	}
	a.reporter(Finding{
		ConcreteNode:      node,
		ConcreteScope:     a.current,
		ConcreteAncestors: append([]cst.Node(nil), a.ancestors...),
		Code:              code,
		Message:           message,
	})
}

func (a *cstAnalyzer) reportRange(code string, start, end int, message string) {
	if !a.enabled[code] || a.reporter == nil {
		return
	}
	a.reporter(Finding{
		ConcreteStart:    start,
		ConcreteEnd:      end,
		HasConcreteRange: true,
		Code:             code,
		Message:          message,
	})
}

func (a *cstAnalyzer) checkFile() {
	a.checkFilenameAndPackage()
	a.checkLinesAndComments()
	a.checkConcreteImports()
}

func (a *cstAnalyzer) packageNameToken() cst.Token {
	tokens := a.tree.Tokens()
	for index, current := range tokens {
		if current.Ch() == token.PACKAGE && index+1 < len(tokens) {
			return tokens[index+1]
		}
	}
	return cst.Token{}
}

func (a *cstAnalyzer) checkFunction(function *cst.FunctionDecl) {
	if function.FunctionName == nil || function.Signature == nil {
		return
	}
	name := function.FunctionName.IDENT
	if name.Src() == "init" {
		a.report("no-init", name, "replace init with explicit initialization")
	}
	a.checkSignature(name, function.Signature.Parameters, function.FunctionBody)
}

func (a *cstAnalyzer) checkMethod(method *cst.MethodDecl) {
	if method.Signature != nil {
		a.checkSignature(method.MethodName, method.Signature.Parameters, method.FunctionBody)
	}
}

func (a *cstAnalyzer) checkSignature(name cst.Token, parameters *cst.Parameters, body cst.Node) {
	count := parameterCount(parameters)
	if count > 5 {
		a.report(
			"max-parameters",
			name,
			fmt.Sprintf("function has %d parameters; maximum is 5", count),
		)
	}
	complexity := cyclomaticComplexity(body)
	if complexity > 10 {
		a.report(
			"cyclomatic-complexity",
			name,
			fmt.Sprintf("function complexity is %d; maximum is 10", complexity),
		)
	}
}

func parameterCount(parameters *cst.Parameters) int {
	if parameters == nil {
		return 0
	}
	count := 0
	for list := parameters.ParameterDeclList; list != nil; list = list.List {
		declaration := list.ParameterDecl
		if declaration == nil || declaration.IdentifierList == nil {
			count++
			continue
		}
		count += declaration.IdentifierList.Len()
	}
	return count
}

func cyclomaticComplexity(body cst.Node) int {
	if body == nil {
		return 0
	}
	complexity := 1
	cst.Walk(body, func(node cst.Node) bool {
		switch current := node.(type) {
		case cst.Token:
			switch current.Ch() {
			case token.IF, token.FOR, token.CASE, token.LAND, token.LOR:
				complexity++
			}
		case *cst.TypeSwitchStmt:
			complexity++
		}
		return true
	})
	return complexity
}

func (a *cstAnalyzer) checkNakedReturn(statement *cst.ReturnStmt) {
	if statement.ExpressionList != nil || !cstEnclosingFunctionHasNamedResults(a.ancestors) {
		return
	}
	a.report("no-naked-return", statement, "return values must be explicit")
}

func cstEnclosingFunctionHasNamedResults(ancestors []cst.Node) bool {
	for index := len(ancestors) - 1; index >= 0; index-- {
		var parameters *cst.Parameters
		switch function := ancestors[index].(type) {
		case *cst.FunctionDecl:
			if function.Signature != nil && function.Signature.Result != nil {
				parameters = function.Signature.Result.Parameters
			}
		case *cst.MethodDecl:
			if function.Signature != nil && function.Signature.Result != nil {
				parameters = function.Signature.Result.Parameters
			}
		case *cst.FunctionLit:
			if function.Signature != nil && function.Signature.Result != nil {
				parameters = function.Signature.Result.Parameters
			}
		default:
			continue
		}
		if parameters == nil {
			return false
		}
		for list := parameters.ParameterDeclList; list != nil; list = list.List {
			if list.ParameterDecl != nil && list.ParameterDecl.IdentifierList != nil {
				return true
			}
		}
		return false
	}
	return false
}

func (a *cstAnalyzer) checkDefer(statement *cst.DeferStmt) {
	for index := len(a.ancestors) - 1; index >= 0; index-- {
		switch a.ancestors[index].(type) {
		case *cst.FunctionDecl, *cst.MethodDecl, *cst.FunctionLit:
			return
		case *cst.ForStmt:
			a.report(
				"no-defer-in-loop",
				statement,
				"defer inside a loop runs at function exit, not iteration exit",
			)
			return
		}
	}
}

func (a *cstAnalyzer) checkElseAfterReturn(statement *cst.IfElseStmt) {
	if !statement.ELSE.IsValid() || statement.Block == nil ||
		!statementListEndsInReturn(statement.Block.StatementList) {
		return
	}
	a.report(
		"no-else-after-return",
		statement.ELSE,
		"remove else and unindent its body after the return",
	)
}

func statementListEndsInReturn(list *cst.StatementList) bool {
	var last cst.Node
	for ; list != nil; list = list.List {
		if list.Statement != nil {
			last = list.Statement
		}
	}
	_, ok := last.(*cst.ReturnStmt)
	return ok
}

func (a *cstAnalyzer) checkPackageVar(declaration *cst.VarDecl) {
	for _, ancestor := range a.ancestors {
		switch ancestor.(type) {
		case *cst.FunctionDecl, *cst.MethodDecl, *cst.FunctionLit:
			return
		}
	}
	cst.Walk(declaration.VarSpec, func(node cst.Node) bool {
		switch spec := node.(type) {
		case *cst.VarSpec:
			if spec.IDENT.Src() != "_" {
				a.report(
					"no-package-var",
					spec.IDENT,
					"package variables introduce mutable global state",
				)
			}
			return false
		case *cst.VarSpec2:
			for names := spec.IdentifierList; names != nil; names = names.List {
				if names.IDENT.Src() != "_" {
					a.report(
						"no-package-var",
						names.IDENT,
						"package variables introduce mutable global state",
					)
				}
			}
			return false
		}
		return true
	})
}
