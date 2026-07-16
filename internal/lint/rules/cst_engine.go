package rules

import (
	"fmt"
	"go/token"

	"github.com/gempir/strider/internal/cst"
)

var cstRuleCodes = map[string]bool{
	"add-constant":                    true,
	"atomic":                          true,
	"bidirectional-control-character": true,
	"banned-characters":               true,
	"argument-limit":                  true,
	"bare-return":                     true,
	"blank-imports":                   true,
	"bool-literal-in-expr":            true,
	"comment-spacings":                true,
	"comments-density":                true,
	"constant-logical-expr":           true,
	"cognitive-complexity":            true,
	"confusing-naming":                true,
	"confusing-results":               true,
	"context-as-argument":             true,
	"cyclomatic-complexity":           true,
	"cyclomatic":                      true,
	"double-negation":                 true,
	"call-to-gc":                      true,
	"deep-exit":                       true,
	"datarace":                        true,
	"defer":                           true,
	"dot-imports":                     true,
	"duplicated-imports":              true,
	"enforce-map-style":               true,
	"enforce-repeated-arg-type-style": true,
	"enforce-slice-style":             true,
	"enforce-switch-style":            true,
	"empty-block":                     true,
	"empty-lines":                     true,
	"early-return":                    true,
	"error-strings":                   true,
	"error-return":                    true,
	"error-naming":                    true,
	"errorf":                          true,
	"epoch-naming":                    true,
	"file-header":                     true,
	"exported":                        true,
	"file-length-limit":               true,
	"flag-parameter":                  true,
	"forbidden-call-in-wg-go":         true,
	"filename-format":                 true,
	"function-length":                 true,
	"function-result-limit":           true,
	"get-return":                      true,
	"identical-branches":              true,
	"identical-ifelseif-branches":     true,
	"identical-ifelseif-conditions":   true,
	"identical-switch-branches":       true,
	"identical-switch-conditions":     true,
	"if-return":                       true,
	"import-alias-naming":             true,
	"import-shadowing":                true,
	"inefficient-map-lookup":          true,
	"imports-blocklist":               true,
	"ineffective-pointer-copy":        true,
	"increment-decrement":             true,
	"indent-error-flow":               true,
	"line-length-limit":               true,
	"max-parameters":                  true,
	"max-control-nesting":             true,
	"max-public-structs":              true,
	"marshal-receiver":                true,
	"modulo-one":                      true,
	"modifies-parameter":              true,
	"modifies-value-receiver":         true,
	"multiline-if-init":               true,
	"nested-structs":                  true,
	"no-defer-in-loop":                true,
	"no-else-after-return":            true,
	"no-init":                         true,
	"no-naked-return":                 true,
	"no-package-var":                  true,
	"optimize-operands-order":         true,
	"package-comments":                true,
	"package-directory-mismatch":      true,
	"package-naming":                  true,
	"redundant-build-tag":             true,
	"redundant-import-alias":          true,
	"receiver-naming":                 true,
	"range":                           true,
	"range-val-address":               true,
	"range-val-in-closure":            true,
	"redefines-builtin-id":            true,
	"redundant-test-main-exit":        true,
	"spaced-compiler-directive":       true,
	"spinning-select-default":         true,
	"string-of-int":                   true,
	"string-format":                   true,
	"struct-tag":                      true,
	"superfluous-else":                true,
	"time-naming":                     true,
	"time-equal":                      true,
	"time-date":                       true,
	"unchecked-type-assertion":        true,
	"unnecessary-format":              true,
	"unnecessary-if":                  true,
	"unnecessary-stmt":                true,
	"unhandled-error":                 true,
	"unreachable-code":                true,
	"unexported-return":               true,
	"unsecure-url-scheme":             true,
	"unexported-naming":               true,
	"use-any":                         true,
	"use-errors-new":                  true,
	"use-fmt-print":                   true,
	"use-slices-sort":                 true,
	"use-waitgroup-go":                true,
	"unused-parameter":                true,
	"unused-receiver":                 true,
	"useless-fallthrough":             true,
	"useless-break":                   true,
	"var-declaration":                 true,
	"var-naming":                      true,
	"waitgroup-by-value":              true,
	"zero-integer-division":           true,
}

// UsesCST reports whether a rule has moved to the concrete-syntax pass.
func UsesCST(code string) bool { return cstRuleCodes[code] }

type cstAnalyzer struct {
	filename      string
	tree          *cst.Tree
	content       []byte
	enabled       map[string]bool
	reporter      func(Finding)
	ancestors     []cst.Node
	current       cst.Node
	receiverNames map[string]string
	marshalKinds  map[string]string
	importNames   map[string]bool
	foldedNames   map[string]map[string]string
	publicStructs int
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
		filename:      input.Filename,
		tree:          input.Tree,
		content:       input.Tree.Source(),
		enabled:       enabled,
		reporter:      input.Report,
		receiverNames: make(map[string]string),
		marshalKinds:  make(map[string]string),
		importNames:   make(map[string]bool),
		foldedNames:   make(map[string]map[string]string),
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
		a.checkConcreteFoldedName("_", current.FunctionName.IDENT)
	case *cst.MethodDecl:
		a.checkMethod(current)
		a.checkConcreteMethodName(current)
	case *cst.ReturnStmt:
		a.checkNakedReturn(current)
	case *cst.DeferStmt:
		a.checkDefer(current)
	case *cst.IfElseStmt:
		a.checkElseAfterReturn(current)
		a.checkConcreteIfElse(current)
	case *cst.IfStmt:
		a.checkConcreteIf(current)
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
		a.checkConcreteAssignmentPolicy(current)
	case *cst.ShortVarDecl:
		a.checkIncrementShortDeclaration(current)
		a.checkConcreteIdentifierList(current.IdentifierList)
		a.checkConcreteShortDeclarationPolicy(current)
	case *cst.PrimaryExpr:
		a.checkConcreteCall(current)
	case *cst.StructType:
		a.checkConcreteStruct(current)
	case *cst.FieldDecl:
		a.checkConcreteStructField(current)
		a.checkConcreteFieldNames(current)
	case *cst.BasicLit:
		a.checkConcreteStringLiteral(current)
	case *cst.Block:
		a.checkConcreteBlock(current)
	case *cst.ForStmt:
		a.checkConcreteControlNesting(current)
		a.checkConcreteFor(current)
	case *cst.SelectStmt:
		a.checkConcreteControlNesting(current)
	case *cst.TypeSwitchStmt:
		a.checkConcreteControlNesting(current)
		a.checkConcreteSwitch(current)
	case *cst.ExprSwitchStmt:
		a.checkConcreteControlNesting(current)
		a.checkConcreteSwitch(current)
	case *cst.TypeAssertion:
		a.checkConcreteTypeAssertion(current)
	case *cst.ParameterDecl:
		a.checkConcreteIdentifierList(current.IdentifierList)
	case *cst.VarSpec:
		a.checkConcreteIdentifier(current.IDENT)
		a.checkConcreteVarSpec(current.IDENT, current.TypeNode, current.ExpressionList)
	case *cst.VarSpec2:
		a.checkConcreteIdentifierList(current.IdentifierList)
		a.checkConcreteVarSpecList(current.IdentifierList, current.TypeNode, current.ExpressionList)
	case *cst.ConstSpec:
		a.checkConcreteIdentifier(current.IDENT)
		a.checkConcreteExportedDeclaration(current.IDENT, current)
	case *cst.ConstSpec2:
		a.checkConcreteIdentifierList(current.IdentifierList)
		a.checkConcreteExportedList(current.IdentifierList, current)
	case *cst.TypeDef:
		a.checkConcreteIdentifier(current.IDENT)
		a.checkConcreteTypeDefinition(current)
	case *cst.AliasDecl:
		a.checkConcreteIdentifier(current.IDENT)
		a.checkConcreteExportedDeclaration(current.IDENT, current)
	case *cst.BreakStmt:
		a.checkConcreteBreak(current)
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
	a.checkConcreteRepeatedLiterals()
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
	a.checkConcreteExportedFunction(name, function, false)
	if name.Src() == "init" {
		a.report("no-init", name, "replace init with explicit initialization")
	}
	a.checkSignature(name, function.Signature.Parameters, function.FunctionBody)
	a.checkConcreteFunctionRules(name, function.Signature, function.FunctionBody, nil)
	a.checkConcreteTestMain(function)
	a.checkConcreteFunctionMutation(function.Signature.Parameters, nil, function.FunctionBody)
}

func (a *cstAnalyzer) checkMethod(method *cst.MethodDecl) {
	if method.Signature != nil {
		a.checkConcreteExportedFunction(method.MethodName, method, true)
		a.checkSignature(method.MethodName, method.Signature.Parameters, method.FunctionBody)
		a.checkConcreteFunctionRules(
			method.MethodName,
			method.Signature,
			method.FunctionBody,
			method.Receiver,
		)
		a.checkConcreteFunctionMutation(
			method.Signature.Parameters,
			method.Receiver,
			method.FunctionBody,
		)
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
	a.report("bare-return", statement, "avoid bare returns; add explicit return expressions")
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
	insideLoop := false
	for index := len(a.ancestors) - 1; index >= 0; index-- {
		switch a.ancestors[index].(type) {
		case *cst.FunctionDecl, *cst.MethodDecl, *cst.FunctionLit:
			index = -1
		case *cst.ForStmt:
			insideLoop = true
			index = -1
		}
	}
	if insideLoop {
		a.report(
			"no-defer-in-loop",
			statement,
			"defer inside a loop runs at function exit, not iteration exit",
		)
	}
	call, ok := statement.Expression.(*cst.PrimaryExpr)
	if !ok {
		return
	}
	if concreteCallName(call) == "recover" {
		a.report("defer", statement, "defer recover() evaluates recover immediately")
	}
	cst.Walk(call.PrimaryExpr, func(node cst.Node) bool {
		literal, ok := node.(*cst.FunctionLit)
		if !ok || literal.Signature == nil || literal.Signature.Result == nil {
			return true
		}
		if concreteDeclCount(concreteResultDecls(literal.Signature.Result)) > 0 {
			a.report("defer", statement, "return values from a deferred function are ignored")
		}
		return false
	})
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
