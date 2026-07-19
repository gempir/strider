package rules

import (
	"fmt"
	"go/token"

	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/diagnostic"
)

var cstRuleCodes = map[string]bool{
	"add-constant":                       true,
	"banned-characters":                  true,
	"bidirectional-control-character":    true,
	"blank-imports":                      true,
	"boolean-literal-comparison":         true,
	"call-to-gc":                         true,
	"cognitive-complexity":               true,
	"confusing-naming":                   true,
	"confusing-results":                  true,
	"constant-logical-expr":              true,
	"context-as-argument":                true,
	"cyclomatic-complexity":              true,
	"deep-exit":                          true,
	"deferred-recover-call":              true,
	"discarded-deferred-result":          true,
	"dot-imports":                        true,
	"double-negation":                    true,
	"duplicated-imports":                 true,
	"early-return":                       true,
	"empty-conditional-block":            true,
	"enforce-switch-style":               true,
	"error-last-result":                  true,
	"error-naming":                       true,
	"error-strings":                      true,
	"exported-declaration-comment":       true,
	"file-length-limit":                  true,
	"filename-format":                    true,
	"flag-parameter":                     true,
	"function-length":                    true,
	"function-result-limit":              true,
	"get-function-return-value":          true,
	"identical-branches":                 true,
	"identical-if-chain-branches":        true,
	"identical-if-chain-conditions":      true,
	"identical-switch-branches":          true,
	"identical-switch-conditions":        true,
	"import-alias-naming":                true,
	"import-shadowing":                   true,
	"imports-blocklist":                  true,
	"increment-decrement":                true,
	"ineffective-pointer-copy":           true,
	"inefficient-map-lookup":             true,
	"insecure-url-scheme":                true,
	"invalid-struct-tag":                 true,
	"marshal-receiver":                   true,
	"max-control-nesting":                true,
	"max-parameters":                     true,
	"max-public-structs":                 true,
	"modifies-parameter":                 true,
	"modifies-value-receiver":            true,
	"modulo-one":                         true,
	"nested-structs":                     true,
	"no-defer-in-loop":                   true,
	"no-else-after-return":               true,
	"no-init":                            true,
	"no-naked-return":                    true,
	"no-package-var":                     true,
	"optimize-operands-order":            true,
	"package-comments":                   true,
	"package-directory-mismatch":         true,
	"package-naming":                     true,
	"prefer-fmt-errorf":                  true,
	"range-value-address":                true,
	"receiver-naming":                    true,
	"redefines-builtin-id":               true,
	"redundant-atomic-result-assignment": true,
	"redundant-build-tag":                true,
	"redundant-error-return-check":       true,
	"redundant-final-return":             true,
	"redundant-import-alias":             true,
	"redundant-switch-break":             true,
	"simplify-range":                     true,
	"single-case-switch":                 true,
	"spaced-compiler-directive":          true,
	"spinning-select-default":            true,
	"string-of-int":                      true,
	"time-date":                          true,
	"time-naming":                        true,
	"unchecked-type-assertion":           true,
	"unexported-naming":                  true,
	"unexported-return":                  true,
	"unnecessary-format":                 true,
	"unnecessary-if":                     true,
	"unreachable-code":                   true,
	"unused-parameter":                   true,
	"unused-receiver":                    true,
	"use-any":                            true,
	"use-errors-new":                     true,
	"use-fmt-print":                      true,
	"use-slices-sort":                    true,
	"var-declaration":                    true,
	"var-naming":                         true,
	"waitgroup-by-value":                 true,
	"zero-integer-division":              true,
}

// UsesCST reports whether a rule has moved to the concrete-syntax pass.
func UsesCST(code string) bool {
	return cstRuleCodes[code]
}

type cstAnalyzer struct {
	filename          string
	tree              *cst.Tree
	content           []byte
	enabled           map[string]bool
	extended          bool
	plan              cstExecutionPlan
	reporter          func(Finding)
	ancestors         []cst.Node
	current           cst.Node
	receiverNames     map[string]string
	marshalKinds      map[string]string
	importNames       map[string]bool
	importPaths       map[string]bool
	importSeen        map[string]bool
	packageName       string
	repeatedLiterals  map[string][]*cst.BasicLit
	functions         []*cstFunctionFacts
	activeFunction    *cstFunctionFacts
	functionDepth     int
	functionBodyDepth int
	foldedNames       map[string]map[string]string
	bannedCharacters  map[rune]bool
	limits            map[string]int
	blockedImports    map[string]bool
	publicStructs     int
}

// AnalyzeCST runs selected native CST rules over one lossless source tree.
func AnalyzeCST(input CSTInput) {
	enabled := make(map[string]bool, len(input.Rules))
	for _, rule := range input.Rules {
		enabled[rule.Meta().Code] = true
	}
	if len(enabled) == 0 || input.Tree == nil {
		return
	}
	extended := false
	for code := range enabled {
		if !defaultCSTCodes[code] {
			extended = true
			break
		}
	}
	plan := compileCSTExecutionPlan(enabled)
	analyzer := &cstAnalyzer{
		filename: input.Filename,
		tree:     input.Tree,
		content:  input.Tree.Bytes(),
		enabled:  enabled,
		extended: extended,
		plan:     plan,
		reporter: input.Report,
		limits:   input.Limits,
	}
	if enabled["imports-blocklist"] {
		analyzer.blockedImports = make(map[string]bool, len(input.BlockedImports))
		for _, path := range input.BlockedImports {
			analyzer.blockedImports[path] = true
		}
	}
	if enabled["banned-characters"] {
		analyzer.bannedCharacters = make(map[rune]bool, len(input.BannedCharacters))
		for _, character := range input.BannedCharacters {
			analyzer.bannedCharacters[character] = true
		}
	}
	if enabled["receiver-naming"] {
		analyzer.receiverNames = make(map[string]string)
	}
	if enabled["marshal-receiver"] {
		analyzer.marshalKinds = make(map[string]string)
	}
	if plan.imports {
		analyzer.importNames = make(map[string]bool)
		analyzer.importPaths = make(map[string]bool)
		analyzer.importSeen = make(map[string]bool)
	}
	if plan.repeatedLiterals {
		analyzer.repeatedLiterals = make(map[string][]*cst.BasicLit)
	}
	if plan.identifiers {
		analyzer.foldedNames = make(map[string]map[string]string)
	}
	if plan.imports || enabled["exported-declaration-comment"] {
		analyzer.packageName = analyzer.packageNameToken().Src()
	}
	if analyzer.extended {
		analyzer.checkFile()
	}
	cst.WalkProductionsWithAncestors(
		input.Tree.Root(),
		func(node cst.Node, ancestors []cst.Node) bool {
			analyzer.ancestors = ancestors
			analyzer.current = node
			analyzer.observe(node, ancestors)
			if analyzer.extended {
				analyzer.check(node)
			} else {
				analyzer.checkDefaults(node)
			}
			return true
		},
	)
	analyzer.finishTraversal()
}

var defaultCSTCodes = map[string]bool{
	"cyclomatic-complexity": true,
	"max-parameters":        true,
	"no-naked-return":       true,
	"no-init":               true,
	"no-package-var":        true,
	"no-defer-in-loop":      true,
	"no-else-after-return":  true,
}

func (a *cstAnalyzer) checkDefaults(node cst.Node) {
	switch current := node.(type) {
	case *cst.FunctionDecl:
		if current.FunctionName == nil {
			return
		}
		name := current.FunctionName.IDENT
		if a.enabled["no-init"] && name.Src() == "init" {
			a.report("no-init", name, "replace init with explicit initialization")
		}
	case *cst.ReturnStmt:
		if a.enabled["no-naked-return"] {
			a.checkNakedReturn(current)
		}
	case *cst.DeferStmt:
		if a.enabled["no-defer-in-loop"] {
			a.checkDefer(current)
		}
	case *cst.IfElseStmt:
		if a.enabled["no-else-after-return"] {
			a.checkElseAfterReturn(current)
		}
	case *cst.VarDecl:
		if a.enabled["no-package-var"] {
			a.checkPackageVar(current)
		}
	}
}

func (a *cstAnalyzer) check(node cst.Node) {
	switch current := node.(type) {
	case *cst.FunctionDecl:
		if current.FunctionName == nil {
			return
		}
		name := current.FunctionName.IDENT
		if a.enabled["no-init"] && name.Src() == "init" {
			a.report("no-init", name, "replace init with explicit initialization")
		}
		if a.plan.identifiers {
			a.checkConcreteFoldedName("_", name)
		}
	case *cst.MethodDecl:
		if a.plan.identifiers {
			a.checkConcreteMethodName(current)
		}
	case *cst.ReturnStmt:
		if a.plan.returns {
			a.checkNakedReturn(current)
		}
	case *cst.DeferStmt:
		if a.plan.defers {
			a.checkDefer(current)
		}
	case *cst.IfElseStmt:
		if a.enabled["no-else-after-return"] {
			a.checkElseAfterReturn(current)
		}
		if a.plan.controlNesting {
			a.checkConcreteControlNesting(current)
		}
		if a.plan.conditionals {
			a.checkConcreteIfElse(current)
		}
	case *cst.IfStmt:
		if a.plan.controlNesting {
			a.checkConcreteControlNesting(current)
		}
		if a.plan.conditionals {
			a.checkConcreteIf(current)
		}
	case *cst.VarDecl:
		if a.plan.packageVars {
			a.checkPackageVar(current)
		}
	case *cst.BinaryExpression:
		if a.plan.binaryExpressions {
			a.checkBinaryExpression(current)
		}
	case *cst.UnaryExpr:
		if a.plan.unaryExpressions {
			a.checkUnaryExpression(current)
		}
	case *cst.InterfaceType:
		if a.plan.interfaces {
			a.checkInterfaceType(current)
		}
	case *cst.Assignment:
		if a.plan.incrementAssignments {
			a.checkIncrementAssignment(current)
		}
		if a.plan.assignmentPolicies {
			a.checkConcreteAssignmentPolicy(current)
		}
	case *cst.ShortVarDecl:
		if a.plan.incrementAssignments {
			a.checkIncrementShortDeclaration(current)
		}
		if a.plan.assignmentPolicies {
			a.checkConcreteShortDeclarationPolicy(current)
		}
		if a.plan.identifiers {
			a.checkConcreteIdentifierList(current.IdentifierList)
		}
	case *cst.PrimaryExpr:
		if a.plan.calls {
			a.checkConcreteCall(current)
		}
	case *cst.StructType:
		if a.plan.structs {
			a.checkConcreteStruct(current)
		}
	case *cst.FieldDecl:
		if a.plan.fields {
			a.checkConcreteStructField(current)
		}
		if a.plan.identifiers {
			a.checkConcreteFieldNames(current)
		}
	case *cst.BasicLit:
		if a.plan.stringLiterals {
			a.checkConcreteStringLiteral(current)
		}
	case *cst.Block:
		if a.plan.blocks {
			a.checkConcreteBlock(current)
		}
	case *cst.ForStmt:
		if a.plan.controlNesting {
			a.checkConcreteControlNesting(current)
		}
		if a.plan.loops {
			a.checkConcreteFor(current)
		}
	case *cst.SelectStmt:
		if a.plan.controlNesting {
			a.checkConcreteControlNesting(current)
		}
	case *cst.TypeSwitchStmt:
		if a.plan.controlNesting {
			a.checkConcreteControlNesting(current)
		}
		if a.plan.switches {
			a.checkConcreteSwitch(current)
		}
	case *cst.ExprSwitchStmt:
		if a.plan.controlNesting {
			a.checkConcreteControlNesting(current)
		}
		if a.plan.switches {
			a.checkConcreteSwitch(current)
		}
	case *cst.TypeAssertion:
		if a.plan.typeAssertions {
			a.checkConcreteTypeAssertion(current)
		}
	case *cst.ParameterDecl:
		if a.plan.identifiers {
			a.checkConcreteIdentifierList(current.IdentifierList)
		}
	case *cst.VarSpec:
		if a.plan.identifiers {
			a.checkConcreteIdentifier(current.IDENT)
		}
		if a.plan.varSpecs {
			a.checkConcreteVarSpec(current.IDENT, current.TypeNode, current.ExpressionList)
		}
	case *cst.VarSpec2:
		if a.plan.identifiers {
			a.checkConcreteIdentifierList(current.IdentifierList)
		}
		if a.plan.varSpecs {
			a.checkConcreteVarSpecList(current.IdentifierList, current.TypeNode, current.ExpressionList)
		}
	case *cst.ConstSpec:
		if a.plan.identifiers {
			a.checkConcreteIdentifier(current.IDENT)
		}
		if a.plan.constSpecs {
			a.checkConcreteExportedDeclaration(current.IDENT, current)
		}
	case *cst.ConstSpec2:
		if a.plan.identifiers {
			a.checkConcreteIdentifierList(current.IdentifierList)
		}
		if a.plan.constSpecs {
			a.checkConcreteExportedList(current.IdentifierList, current)
		}
	case *cst.TypeDef:
		if a.plan.identifiers {
			a.checkConcreteIdentifier(current.IDENT)
		}
		if a.plan.typeDefinitions {
			a.checkConcreteTypeDefinition(current)
		}
	case *cst.AliasDecl:
		if a.plan.identifiers {
			a.checkConcreteIdentifier(current.IDENT)
		}
		if a.plan.constSpecs {
			a.checkConcreteExportedDeclaration(current.IDENT, current)
		}
	case *cst.ImportSpec:
		if a.plan.imports {
			a.checkConcreteImport(current)
		}
	case *cst.BreakStmt:
		if a.plan.breaks {
			a.checkConcreteBreak(current)
		}
	}
}

func (a *cstAnalyzer) report(code string, node cst.Node, message string) {
	if !a.enabled[code] || node == nil || a.reporter == nil {
		return
	}
	a.reporter(Finding{ConcreteNode: node, Code: code, Message: message})
}

func (a *cstAnalyzer) reportFix(code string, node cst.Node, message string, fix diagnostic.Fix) {
	if !a.enabled[code] || node == nil || a.reporter == nil {
		return
	}
	a.reporter(Finding{ConcreteNode: node, Code: code, Message: message, Fixes: []diagnostic.Fix{fix}})
}

func (a *cstAnalyzer) reportRange(code string, start, end int, message string) {
	if !a.enabled[code] || a.reporter == nil {
		return
	}
	a.reporter(Finding{ConcreteStart: start, ConcreteEnd: end, HasConcreteRange: true, Code: code, Message: message})
}

func (a *cstAnalyzer) checkFile() {
	if a.plan.filename {
		a.checkFilenameAndPackage()
	}
	if a.plan.comments {
		a.checkLinesAndComments()
	}
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

func (a *cstAnalyzer) finishTraversal() {
	if a.plan.repeatedLiterals {
		a.finishConcreteRepeatedLiterals()
	}
	for _, facts := range a.functions {
		switch function := facts.node.(type) {
		case *cst.FunctionDecl:
			a.checkFunction(function, facts)
		case *cst.MethodDecl:
			a.checkMethod(function, facts)
		}
	}
}

func (a *cstAnalyzer) checkFunction(function *cst.FunctionDecl, facts *cstFunctionFacts) {
	if function.FunctionName == nil || function.Signature == nil {
		return
	}
	name := function.FunctionName.IDENT
	if a.enabled["exported-declaration-comment"] {
		a.checkConcreteExportedFunction(name, function, false)
	}
	a.checkSignature(name, function.Signature.Parameters, facts.complexity)
	if a.extended {
		a.checkConcreteFunctionRules(name, function.Signature, function.FunctionBody, nil, facts)
		if a.enabled["modifies-parameter"] {
			a.checkConcreteFunctionMutation(function.Signature.Parameters, nil, function.FunctionBody)
		}
	}
}

func (a *cstAnalyzer) checkMethod(method *cst.MethodDecl, facts *cstFunctionFacts) {
	if method.Signature != nil {
		if a.enabled["exported-declaration-comment"] {
			a.checkConcreteExportedFunction(method.MethodName, method, true)
		}
		a.checkSignature(method.MethodName, method.Signature.Parameters, facts.complexity)
		if a.extended {
			a.checkConcreteFunctionRules(method.MethodName, method.Signature, method.FunctionBody, method.Receiver, facts)
			if a.enabled["modifies-parameter"] || a.enabled["modifies-value-receiver"] {
				a.checkConcreteFunctionMutation(method.Signature.Parameters, method.Receiver, method.FunctionBody)
			}
		}
	}
}

func (a *cstAnalyzer) checkSignature(name cst.Token, parameters *cst.Parameters, complexity int) {
	if a.enabled["max-parameters"] {
		count := parameterCount(parameters)
		limit := a.limit("max-parameters", 8)
		if count > limit {
			a.report("max-parameters", name, fmt.Sprintf("function has %d parameters; maximum is %d", count, limit))
		}
	}
	if a.enabled["cyclomatic-complexity"] {
		if complexity > 10 {
			a.report("cyclomatic-complexity", name, fmt.Sprintf("function complexity is %d; maximum is 10", complexity))
		}
	}
}

func (a *cstAnalyzer) limit(code string, fallback int) int {
	if configured := a.limits[code]; configured > 0 {
		return configured
	}
	return fallback
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
		a.report("no-defer-in-loop", statement, "defer inside a loop runs at function exit, not iteration exit")
	}
	if !a.enabled["deferred-recover-call"] && !a.enabled["discarded-deferred-result"] {
		return
	}
	call, ok := statement.Expression.(*cst.PrimaryExpr)
	if !ok {
		return
	}
	if a.enabled["deferred-recover-call"] && concreteCallName(call) == "recover" {
		a.report("deferred-recover-call", statement, "defer recover() evaluates recover immediately")
	}
	if !a.enabled["discarded-deferred-result"] {
		return
	}
	cst.Walk(
		call.PrimaryExpr,
		func(node cst.Node) bool {
			literal,
				ok := node.(*cst.FunctionLit)
			if !ok || literal.Signature == nil || literal.Signature.Result == nil {
				return true
			}
			if concreteDeclCount(concreteResultDecls(literal.Signature.Result)) > 0 {
				a.report("discarded-deferred-result", statement, "return values from a deferred function are ignored")
			}
			return false
		},
	)
}

func (a *cstAnalyzer) checkElseAfterReturn(statement *cst.IfElseStmt) {
	if !statement.ELSE.IsValid() || statement.Block == nil || !statementListEndsInReturn(statement.Block.StatementList) {
		return
	}
	a.report("no-else-after-return", statement.ELSE, "remove else and unindent its body after the return")
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
	cst.Walk(
		declaration.VarSpec,
		func(node cst.Node) bool {
			switch spec := node.(type) {
			case *cst.VarSpec:
				if spec.IDENT.Src() != "_" {
					a.report("no-package-var", spec.IDENT, "package variables introduce mutable global state")
				}
				return false
			case *cst.VarSpec2:
				for names := spec.IdentifierList; names != nil; names = names.List {
					if names.IDENT.Src() != "_" {
						a.report("no-package-var", names.IDENT, "package variables introduce mutable global state")
					}
				}
				return false
			}
			return true
		},
	)
}
