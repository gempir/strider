package rules

import "go/ast"

func (a *analyzer) checkNode(node ast.Node) {
	switch n := node.(type) {
	case *ast.FuncDecl:
		a.checkFunction(n)
	case *ast.GenDecl:
		a.checkDeclaration(n)
	case *ast.AssignStmt:
		a.checkAssignment(n)
	case *ast.CallExpr:
		a.checkCall(n)
	case *ast.IfStmt:
		a.checkIf(n)
	case *ast.ForStmt:
		a.checkFor(n)
	case *ast.RangeStmt:
		a.checkRange(n)
	case *ast.SwitchStmt:
		a.checkSwitch(n.Body)
	case *ast.TypeSwitchStmt:
		a.checkSwitch(n.Body)
	case *ast.BlockStmt:
		a.checkBlock(n)
	case *ast.BinaryExpr:
		a.checkBinary(n)
	case *ast.DeferStmt:
		a.checkDefer(n)
	case *ast.TypeAssertExpr:
		a.checkTypeAssertion(n)
	case *ast.IncDecStmt:
		a.checkIncDec(n)
	case *ast.BranchStmt:
		a.checkBranch(n)
	case *ast.StructType:
		a.checkStruct(n)
	case *ast.InterfaceType:
		a.checkUseAny(n)
	case *ast.BasicLit:
		a.checkLiteral(n)
	case *ast.ReturnStmt:
		a.checkReturn(n)
	}
}
