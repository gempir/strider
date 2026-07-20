package rules

import "github.com/gempir/strider/internal/cst"

var (
	filenameBehavior = behavior([]NodeKind{
		fileNodeKind,
	}, func(pass *Pass, _ cst.Node) {
		pass.checkFilenameAndPackage()
	})
	commentBehavior = behavior([]NodeKind{
		fileNodeKind,
	}, func(pass *Pass, _ cst.Node) {
		pass.checkLinesAndComments()
	})
	noInitBehavior = behavior(
		[]NodeKind{
			"FunctionDecl",
		},
		func(pass *Pass, node cst.Node) {
			function := node.(*cst.FunctionDecl)
			if function.FunctionName != nil && function.FunctionName.IDENT.Src() == "init" {
				pass.report("no-init", function.FunctionName.IDENT, "replace init with explicit initialization")
			}
		},
	)
	nakedReturnBehavior = behavior([]NodeKind{
		"ReturnStmt",
	}, func(pass *Pass, node cst.Node) {
		pass.checkNakedReturn(node.(*cst.ReturnStmt))
	})
	noElseAfterReturnBehavior = behavior([]NodeKind{
		"IfElseStmt",
	}, func(pass *Pass, node cst.Node) {
		pass.checkElseAfterReturn(node.(*cst.IfElseStmt))
	})
	packageVarBehavior = behavior([]NodeKind{
		"VarDecl",
	}, func(pass *Pass, node cst.Node) {
		pass.checkPackageVar(node.(*cst.VarDecl))
	})
	functionBehavior = behavior([]NodeKind{
		"FunctionDecl",
		"MethodDecl",
	}, inspectFunctionCheck)
	exportedDeclarationBehavior = behavior(
		[]NodeKind{
			"FunctionDecl",
			"MethodDecl",
			"VarSpec",
			"VarSpec2",
			"ConstSpec",
			"ConstSpec2",
			"TypeDef",
			"AliasDecl",
		},
		inspectExportedDeclarationCheck,
	)
	timeNamingBehavior = behavior([]NodeKind{
		"FunctionDecl",
		"MethodDecl",
		"VarSpec",
		"VarSpec2",
	}, inspectTimeNamingCheck)
	importBehavior = behavior([]NodeKind{
		"ImportSpec",
	}, func(pass *Pass, node cst.Node) {
		pass.checkImport(node.(*cst.ImportSpec))
	})
	importShadowingBehavior = behavior(
		[]NodeKind{
			"ImportSpec",
			"FunctionDecl",
			"MethodDecl",
			"ShortVarDecl",
			"FieldDecl",
			"ParameterDecl",
			"VarSpec",
			"VarSpec2",
			"ConstSpec",
			"ConstSpec2",
			"TypeDef",
			"AliasDecl",
		},
		inspectImportShadowingCheck,
	)
	invalidStructTagBehavior = behavior(
		[]NodeKind{
			"ImportSpec",
			"FieldDecl",
		},
		func(pass *Pass, node cst.Node) {
			switch current := node.(type) {
			case *cst.ImportSpec:
				pass.checkImport(current)
			case *cst.FieldDecl:
				pass.checkStructField(current)
			}
		},
	)
	repeatedLiteralBehavior = behavior([]NodeKind{
		"BasicLit",
		finishNodeKind,
	}, inspectRepeatedLiteralCheck)
	identifierBehavior = behavior(
		[]NodeKind{
			"FunctionDecl",
			"MethodDecl",
			"ShortVarDecl",
			"FieldDecl",
			"ParameterDecl",
			"VarSpec",
			"VarSpec2",
			"ConstSpec",
			"ConstSpec2",
			"TypeDef",
			"AliasDecl",
		},
		inspectIdentifierCheck,
	)
	deferBehavior = behavior([]NodeKind{
		"DeferStmt",
	}, func(pass *Pass, node cst.Node) {
		pass.checkDefer(node.(*cst.DeferStmt))
	})
	conditionalBehavior = behavior(
		[]NodeKind{
			"IfStmt",
			"IfElseStmt",
		},
		func(pass *Pass, node cst.Node) {
			switch current := node.(type) {
			case *cst.IfStmt:
				pass.checkIf(current)
			case *cst.IfElseStmt:
				pass.checkIfElse(current)
			}
		},
	)
	controlNestingBehavior = behavior(
		[]NodeKind{
			"IfStmt",
			"IfElseStmt",
			"ForStmt",
			"SelectStmt",
			"TypeSwitchStmt",
			"ExprSwitchStmt",
		},
		func(pass *Pass, node cst.Node) {
			pass.checkControlNesting(node)
		},
	)
	switchBehavior = behavior([]NodeKind{
		"TypeSwitchStmt",
		"ExprSwitchStmt",
	}, func(pass *Pass, node cst.Node) {
		pass.checkSwitch(node)
	})
	varSpecBehavior = behavior([]NodeKind{
		"VarSpec",
		"VarSpec2",
	}, inspectVarSpecCheck)
	assignmentBehavior = behavior(
		[]NodeKind{
			"Assignment",
			"ShortVarDecl",
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
			"Assignment",
			"ShortVarDecl",
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
	loopBehavior = behavior([]NodeKind{
		"ForStmt",
	}, func(pass *Pass, node cst.Node) {
		pass.checkFor(node.(*cst.ForStmt))
	})
	blockBehavior = behavior([]NodeKind{
		"Block",
	}, func(pass *Pass, node cst.Node) {
		pass.checkBlock(node.(*cst.Block))
	})
	binaryBehavior = behavior([]NodeKind{
		"BinaryExpression",
	}, func(pass *Pass, node cst.Node) {
		pass.checkBinaryExpression(node.(*cst.BinaryExpression))
	})
	unaryBehavior = behavior([]NodeKind{
		"UnaryExpr",
	}, func(pass *Pass, node cst.Node) {
		pass.checkUnaryExpression(node.(*cst.UnaryExpr))
	})
	callBehavior = behavior([]NodeKind{
		"PrimaryExpr",
	}, func(pass *Pass, node cst.Node) {
		pass.checkCall(node.(*cst.PrimaryExpr))
	})
	structBehavior = behavior([]NodeKind{
		"StructType",
	}, func(pass *Pass, node cst.Node) {
		pass.checkStruct(node.(*cst.StructType))
	})
	interfaceBehavior = behavior([]NodeKind{
		"InterfaceType",
	}, func(pass *Pass, node cst.Node) {
		pass.checkInterfaceType(node.(*cst.InterfaceType))
	})
	typeAssertionBehavior = behavior([]NodeKind{
		"TypeAssertion",
	}, func(pass *Pass, node cst.Node) {
		pass.checkTypeAssertion(node.(*cst.TypeAssertion))
	})
	typeDefinitionBehavior = behavior([]NodeKind{
		"TypeDef",
	}, func(pass *Pass, node cst.Node) {
		pass.checkTypeDefinition(node.(*cst.TypeDef))
	})
	breakBehavior = behavior([]NodeKind{
		"BreakStmt",
	}, func(pass *Pass, node cst.Node) {
		pass.checkBreak(node.(*cst.BreakStmt))
	})
	stringLiteralBehavior = behavior([]NodeKind{
		"BasicLit",
	}, func(pass *Pass, node cst.Node) {
		pass.checkStringLiteral(node.(*cst.BasicLit))
	})
)

func behavior(interests []NodeKind, inspect func(*Pass, cst.Node)) syntaxBehavior {
	return syntaxBehavior{
		interests: interests,
		inspect:   inspect,
	}
}

func inspectFunctionCheck(pass *Pass, node cst.Node) {
	facts := pass.functionFacts(node)
	switch current := node.(type) {
	case *cst.FunctionDecl:
		pass.checkFunction(current, facts)
	case *cst.MethodDecl:
		pass.checkMethod(current, facts)
	}
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
		pass.checkTypeDefinition(current)
	case *cst.AliasDecl:
		pass.checkExportedDeclaration(current.IDENT, current)
	}
}

func inspectTimeNamingCheck(pass *Pass, node cst.Node) {
	switch current := node.(type) {
	case *cst.FunctionDecl, *cst.MethodDecl:
		inspectFunctionCheck(pass, current)
	case *cst.VarSpec:
		pass.checkVarSpec(current.IDENT, current.TypeNode, current.ExpressionList)
	case *cst.VarSpec2:
		pass.checkVarSpecList(current.IdentifierList, current.TypeNode, current.ExpressionList)
	}
}

func inspectImportShadowingCheck(pass *Pass, node cst.Node) {
	if spec, ok := node.(*cst.ImportSpec); ok {
		pass.checkImport(spec)
		return
	}
	inspectIdentifierCheck(pass, node)
}

func inspectIdentifierCheck(pass *Pass, node cst.Node) {
	switch current := node.(type) {
	case *cst.FunctionDecl:
		if current.FunctionName != nil {
			pass.checkFoldedName("_", current.FunctionName.IDENT)
		}
	case *cst.MethodDecl:
		pass.checkMethodName(current)
	case *cst.ShortVarDecl:
		pass.checkIdentifierList(current.IdentifierList)
	case *cst.FieldDecl:
		pass.checkFieldNames(current)
	case *cst.ParameterDecl:
		pass.checkIdentifierList(current.IdentifierList)
	case *cst.VarSpec:
		pass.checkIdentifier(current.IDENT)
	case *cst.VarSpec2:
		pass.checkIdentifierList(current.IdentifierList)
	case *cst.ConstSpec:
		pass.checkIdentifier(current.IDENT)
	case *cst.ConstSpec2:
		pass.checkIdentifierList(current.IdentifierList)
	case *cst.TypeDef:
		pass.checkIdentifier(current.IDENT)
	case *cst.AliasDecl:
		pass.checkIdentifier(current.IDENT)
	}
}

func inspectVarSpecCheck(pass *Pass, node cst.Node) {
	switch current := node.(type) {
	case *cst.VarSpec:
		pass.checkVarSpec(current.IDENT, current.TypeNode, current.ExpressionList)
	case *cst.VarSpec2:
		pass.checkVarSpecList(current.IdentifierList, current.TypeNode, current.ExpressionList)
	}
}

func inspectRepeatedLiteralCheck(pass *Pass, node cst.Node) {
	if node == nil {
		pass.finishRepeatedLiterals()
		return
	}
	pass.observeRepeatedLiteral(node.(*cst.BasicLit), pass.ancestors)
}
