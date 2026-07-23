package syntax

import "github.com/gempir/strider/internal/cst"

var (
	documentationPeriodBehavior = behaviorWithStart(
		[]NodeKind{
			nodeFunctionDecl,
			nodeMethodDecl,
			nodeVarSpec,
			nodeVarSpec2,
			nodeConstSpec,
			nodeConstSpec2,
			nodeTypeDef,
			nodeAliasDecl,
		},
		func(pass *Pass) {
			pass.checkDocumentationPeriod(nil)
		},
		func(pass *Pass, node cst.Node) {
			pass.checkDocumentationPeriod(node)
		},
	)
	topLevelDeclarationOrderBehavior  = nodeCheck(nodeSourceFile, (*Pass).checkTopLevelDeclarationOrder)
	excessiveBlankIdentifiersBehavior = behavior([]NodeKind{
		nodeAssignment,
		nodeShortVarDecl,
	}, func(pass *Pass, node cst.Node) {
		pass.checkExcessiveBlankIdentifiers(node)
	})
	noInitBehavior = nodeCheck(
		nodeFunctionDecl,
		func(pass *Pass, function *cst.FunctionDecl) {
			if function.FunctionName != nil && function.FunctionName.IDENT.Src() == "init" {
				pass.Report(function.FunctionName.IDENT, "replace init with explicit initialization")
			}
		},
	)
	nakedReturnBehavior         = nodeCheck(nodeReturnStmt, (*Pass).checkNakedReturn)
	noElseAfterReturnBehavior   = nodeCheck(nodeIfElseStmt, (*Pass).checkElseAfterReturn)
	packageVarBehavior          = nodeCheck(nodeVarDecl, (*Pass).checkPackageVar)
	exportedDeclarationBehavior = behavior(
		[]NodeKind{
			nodeFunctionDecl,
			nodeMethodDecl,
			nodeVarSpec,
			nodeVarSpec2,
			nodeConstSpec,
			nodeConstSpec2,
			nodeTypeDef,
			nodeAliasDecl,
		},
		inspectExportedDeclarationCheck,
	)
	timeNamingBehavior = behavior([]NodeKind{
		nodeFunctionDecl,
		nodeMethodDecl,
		nodeVarSpec,
		nodeVarSpec2,
	}, inspectTimeNamingCheck)
	importShadowingBehavior = behavior(
		[]NodeKind{
			nodeImportSpec,
			nodeFunctionDecl,
			nodeMethodDecl,
			nodeShortVarDecl,
			nodeFieldDecl,
			nodeParameterDecl,
			nodeVarSpec,
			nodeVarSpec2,
			nodeConstSpec,
			nodeConstSpec2,
			nodeTypeDef,
			nodeAliasDecl,
		},
		inspectImportShadowingCheck,
	)
	invalidStructTagBehavior = behavior(
		[]NodeKind{
			nodeImportSpec,
			nodeFieldDecl,
		},
		func(pass *Pass, node cst.Node) {
			switch current := node.(type) {
			case *cst.ImportSpec:
				pass.observeImport(current)
			case *cst.FieldDecl:
				pass.checkStructField(current)
			}
		},
	)
	repeatedLiteralBehavior = behaviorWithFinish([]NodeKind{
		nodeBasicLit,
	}, inspectRepeatedLiteralCheck, func(pass *Pass) {
		inspectRepeatedLiteralCheck(pass, nil)
	})
	controlNestingBehavior = behavior(
		[]NodeKind{
			nodeIfStmt,
			nodeIfElseStmt,
			nodeForStmt,
			nodeSelectStmt,
			nodeTypeSwitchStmt,
			nodeExprSwitchStmt,
		},
		func(pass *Pass, node cst.Node) {
			pass.checkControlNesting(node)
		},
	)
	assignmentBehavior = behavior(
		[]NodeKind{
			nodeAssignment,
			nodeShortVarDecl,
		},
		func(pass *Pass, node cst.Node) {
			switch current := node.(type) {
			case *cst.Assignment:
				pass.checkAssignmentPolicy(current)
			case *cst.ShortVarDecl:
				pass.checkShortDeclarationPolicy(current)
			}
		},
	)
	incrementBehavior = behavior(
		[]NodeKind{
			nodeAssignment,
			nodeShortVarDecl,
		},
		func(pass *Pass, node cst.Node) {
			switch current := node.(type) {
			case *cst.Assignment:
				pass.checkIncrementAssignment(current)
			case *cst.ShortVarDecl:
				pass.checkIncrementShortDeclaration(current)
			}
		},
	)
	structBehavior         = nodeCheck(nodeStructType, (*Pass).checkStruct)
	interfaceBehavior      = nodeCheck(nodeInterfaceType, (*Pass).checkInterfaceType)
	typeAssertionBehavior  = nodeCheck(nodeTypeAssertion, (*Pass).checkTypeAssertion)
	typeDefinitionBehavior = nodeCheck(nodeTypeDef, (*Pass).checkMaxPublicStructs)
	breakBehavior          = nodeCheck(nodeBreakStmt, (*Pass).checkBreak)
	stringLiteralBehavior  = nodeCheck(nodeBasicLit, (*Pass).checkStringLiteral)
)

func behavior(interests []NodeKind, inspect func(*Pass, cst.Node)) syntaxBehavior {
	return syntaxBehavior{
		interests: interests,
		inspect:   inspect,
	}
}

func nodeCheck[T cst.Node](interest NodeKind, inspect func(*Pass, T)) syntaxBehavior {
	return behavior([]NodeKind{
		interest,
	}, func(pass *Pass, node cst.Node) {
		typed, ok := node.(T)
		if ok {
			inspect(pass, typed)
		}
	})
}

func startBehavior(start func(*Pass)) syntaxBehavior {
	return syntaxBehavior{
		start: start,
	}
}

func behaviorWithStart(interests []NodeKind, start func(*Pass), inspect func(*Pass, cst.Node)) syntaxBehavior {
	result := behavior(interests, inspect)
	result.start = start
	return result
}

func behaviorWithFinish(interests []NodeKind, inspect func(*Pass, cst.Node), finish func(*Pass)) syntaxBehavior {
	result := behavior(interests, inspect)
	result.finish = finish
	return result
}

func callCheck(inspect func(*Pass, callFacts)) syntaxBehavior {
	return nodeCheck(nodePrimaryExpr, func(pass *Pass, call *cst.PrimaryExpr) {
		inspect(pass, pass.callFacts(call))
	})
}

func functionCheck(inspect func(*Pass, *functionFacts)) syntaxBehavior {
	return behavior([]NodeKind{
		nodeFunctionDecl,
		nodeMethodDecl,
	}, func(pass *Pass, node cst.Node) {
		inspect(pass, pass.functionFacts(node))
	})
}

func importCheck(inspect func(*Pass, *cst.ImportSpec)) syntaxBehavior {
	return nodeCheck(nodeImportSpec, inspect)
}

func deferCheck(inspect func(*Pass, *cst.DeferStmt)) syntaxBehavior {
	return nodeCheck(nodeDeferStmt, inspect)
}

func binaryCheck(inspect func(*Pass, *cst.BinaryExpression)) syntaxBehavior {
	return nodeCheck(nodeBinaryExpression, inspect)
}

func unaryCheck(inspect func(*Pass, *cst.UnaryExpr)) syntaxBehavior {
	return nodeCheck(nodeUnaryExpr, inspect)
}

func conditionalCheck(inspect func(*Pass, conditional)) syntaxBehavior {
	return behavior(
		[]NodeKind{
			nodeIfStmt,
			nodeIfElseStmt,
		},
		func(pass *Pass, node cst.Node) {
			switch current := node.(type) {
			case *cst.IfStmt:
				if statement, ok := conditionalFromIf(current); ok {
					inspect(pass, statement)
				}
			case *cst.IfElseStmt:
				if statement, ok := conditionalFromIfElse(current); ok {
					inspect(pass, statement)
				}
			}
		},
	)
}

func blockCheck(inspect func(*Pass, *cst.Block)) syntaxBehavior {
	return nodeCheck(nodeBlock, inspect)
}

func forCheck(inspect func(*Pass, *cst.ForStmt)) syntaxBehavior {
	return nodeCheck(nodeForStmt, inspect)
}

func switchCheck(inspect func(*Pass, cst.Node)) syntaxBehavior {
	return behavior([]NodeKind{
		nodeTypeSwitchStmt,
		nodeExprSwitchStmt,
	}, inspect)
}

func identifierCheck(inspect func(*Pass, cst.Token)) syntaxBehavior {
	return behavior(
		[]NodeKind{
			nodeShortVarDecl,
			nodeFieldDecl,
			nodeParameterDecl,
			nodeVarSpec,
			nodeVarSpec2,
			nodeConstSpec,
			nodeConstSpec2,
			nodeTypeDef,
			nodeAliasDecl,
		},
		func(pass *Pass, node cst.Node) {
			inspectDeclaredIdentifiers(pass, node, inspect)
		},
	)
}

func varSpecCheck(inspect func(*Pass, cst.Token, cst.Node, *cst.ExpressionList)) syntaxBehavior {
	return behavior(
		[]NodeKind{
			nodeVarSpec,
			nodeVarSpec2,
		},
		func(pass *Pass, node cst.Node) {
			switch current := node.(type) {
			case *cst.VarSpec:
				inspect(pass, current.IDENT, current.TypeNode, current.ExpressionList)
			case *cst.VarSpec2:
				pass.inspectVarSpecList(current.IdentifierList, current.TypeNode, current.ExpressionList, inspect)
			}
		},
	)
}

func inspectExportedDeclarationCheck(pass *Pass, node cst.Node) {
	switch current := node.(type) {
	case *cst.FunctionDecl:
		if current.FunctionName != nil {
			pass.checkExportedFunction(current.FunctionName.IDENT, current, false)
		}
	case *cst.MethodDecl:
		pass.checkExportedFunction(current.MethodName, current, true)
	case *cst.VarSpec:
		pass.checkExportedDeclaration(current.IDENT, current)
	case *cst.VarSpec2:
		pass.checkExportedList(current.IdentifierList, current)
	case *cst.ConstSpec:
		pass.checkExportedDeclaration(current.IDENT, current)
	case *cst.ConstSpec2:
		pass.checkExportedList(current.IdentifierList, current)
	case *cst.TypeDef:
		pass.checkExportedDeclaration(current.IDENT, current)
	case *cst.AliasDecl:
		pass.checkExportedDeclaration(current.IDENT, current)
	}
}

func inspectTimeNamingCheck(pass *Pass, node cst.Node) {
	switch current := node.(type) {
	case *cst.FunctionDecl, *cst.MethodDecl:
		pass.checkTimeNaming(pass.functionFacts(current))
	case *cst.VarSpec:
		pass.checkTimeVariableNaming(current.IDENT, current.TypeNode, current.ExpressionList)
	case *cst.VarSpec2:
		pass.inspectVarSpecList(current.IdentifierList, current.TypeNode, current.ExpressionList, (*Pass).checkTimeVariableNaming)
	}
}

func inspectImportShadowingCheck(pass *Pass, node cst.Node) {
	if spec, ok := node.(*cst.ImportSpec); ok {
		pass.observeImport(spec)
		return
	}
	inspectDeclaredIdentifiers(pass, node, (*Pass).checkImportShadowing)
}

func inspectConfusingNamingCheck(pass *Pass, node cst.Node) {
	switch current := node.(type) {
	case *cst.FunctionDecl:
		if current.FunctionName != nil {
			pass.checkFoldedName("_", current.FunctionName.IDENT)
		}
	case *cst.MethodDecl:
		pass.checkMethodName(current)
	case *cst.FieldDecl:
		pass.checkFieldNames(current)
	}
}

func inspectDeclaredIdentifiers(pass *Pass, node cst.Node, inspect func(*Pass, cst.Token)) {
	var names []cst.Token
	switch current := node.(type) {
	case *cst.ShortVarDecl:
		names = identifierTokens(current.IdentifierList)
	case *cst.FieldDecl:
		names = identifierTokens(current.IdentifierList)
	case *cst.ParameterDecl:
		names = identifierTokens(current.IdentifierList)
	case *cst.VarSpec:
		names = []cst.Token{
			current.IDENT,
		}
	case *cst.VarSpec2:
		names = identifierTokens(current.IdentifierList)
	case *cst.ConstSpec:
		names = []cst.Token{
			current.IDENT,
		}
	case *cst.ConstSpec2:
		names = identifierTokens(current.IdentifierList)
	case *cst.TypeDef:
		names = []cst.Token{
			current.IDENT,
		}
	case *cst.AliasDecl:
		names = []cst.Token{
			current.IDENT,
		}
	}
	for _, name := range names {
		inspect(pass, name)
	}
}

func inspectRepeatedLiteralCheck(pass *Pass, node cst.Node) {
	if node == nil {
		pass.finishRepeatedLiterals()
		return
	}
	literal, ok := node.(*cst.BasicLit)
	if ok {
		pass.observeRepeatedLiteral(literal, pass.ancestors)
	}
}
