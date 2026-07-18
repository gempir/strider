package semantic

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type singleIterationLoopRule struct{}

func (singleIterationLoopRule) Meta() Meta {
	return Meta{
		Code:            "single-iteration-loop",
		Summary:         "detect loops that always exit during their first iteration",
		Explanation:     "A loop whose top-level control flow always returns or breaks after a conditional branch cannot begin a second iteration. This usually means the exit was placed outside the intended branch.",
		GoodExample:     "for _, value := range values { if done(value) { break }; use(value) }",
		BadExample:      "for _, value := range values { if done(value) { use(value) }; return }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (singleIterationLoopRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			analyzeFunctionLoops(pass, function.Body)
		}
	}
}

func analyzeFunctionLoops(pass *Pass, body *ast.BlockStmt) {
	labels := make(map[types.Object]ast.Stmt)
	inspectWithoutClosures(
		body,
		func(node ast.Node) bool {
			label,
				ok := node.(*ast.LabeledStmt)
			if ok {
				labels[pass.TypesInfo.ObjectOf(label.Label)] = label.Stmt
			}
			return true
		},
	)

	inspectWithoutClosures(
		body,
		func(node ast.Node) bool {
			var loop ast.Stmt
			var loopBody *ast.BlockStmt
			switch node := node.(type) {
			case *ast.ForStmt:
				loop = node
				loopBody = node.Body
			case *ast.RangeStmt:
				if !orderedRangeType(pass.TypesInfo.TypeOf(node.X)) {
					return true
				}
				loop = node
				loopBody = node.Body
			default:
				return true
			}
			exit := topLevelLoopExit(pass, loop, loopBody, labels)
			if exit != nil && !loopHasEscape(pass, loop, loopBody, labels) {
				pass.Report(
					exit,
					"the surrounding loop always terminates during its first iteration",
				)
			}
			return true
		},
	)

	ast.Inspect(
		body,
		func(node ast.Node) bool {
			closure,
				ok := node.(*ast.FuncLit)
			if !ok {
				return true
			}
			analyzeFunctionLoops(pass, closure.Body)
			return false
		},
	)
}

func inspectWithoutClosures(root ast.Node, visit func(ast.Node) bool) {
	ast.Inspect(
		root,
		func(node ast.Node) bool {
			if _,
				ok := node.(*ast.FuncLit); ok {
				return false
			}
			return visit(node)
		},
	)
}

func orderedRangeType(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	valueType = types.Unalias(valueType)
	if parameter, ok := valueType.(*types.TypeParam); ok {
		return orderedRangeType(parameter.Constraint())
	}
	switch underlying := valueType.Underlying().(type) {
	case *types.Slice, *types.Chan, *types.Basic, *types.Pointer, *types.Array:
		return true
	case *types.Map, *types.Signature:
		return false
	case *types.Interface:
		if underlying.NumEmbeddeds() == 0 {
			return false
		}
		for index := range underlying.NumEmbeddeds() {
			if !orderedRangeType(underlying.EmbeddedType(index)) {
				return false
			}
		}
		return true
	case *types.Union:
		for index := range underlying.Len() {
			if !orderedRangeType(underlying.Term(index).Type()) {
				return false
			}
		}
		return underlying.Len() != 0
	default:
		return false
	}
}

func topLevelLoopExit(
	pass *Pass,
	loop ast.Stmt,
	body *ast.BlockStmt,
	labels map[types.Object]ast.Stmt,
) ast.Node {
	if len(body.List) < 2 {
		return nil
	}
	var exit ast.Node
	hasBranching := false
	for _, statement := range body.List {
		switch statement := statement.(type) {
		case *ast.BranchStmt:
			switch statement.Tok {
			case token.BREAK:
				if statement.Label == nil || labels[pass.TypesInfo.ObjectOf(statement.Label)] == loop {
					exit = statement
				}
			case token.CONTINUE:
				if statement.Label == nil || labels[pass.TypesInfo.ObjectOf(statement.Label)] == loop {
					return nil
				}
			}
		case *ast.ReturnStmt:
			exit = statement
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.SelectStmt:
			hasBranching = true
		}
	}
	if !hasBranching {
		return nil
	}
	return exit
}

func loopHasEscape(
	pass *Pass,
	loop ast.Stmt,
	body *ast.BlockStmt,
	labels map[types.Object]ast.Stmt,
) bool {
	hasEscape := false
	inspectWithoutClosures(
		body,
		func(node ast.Node) bool {
			branch,
				ok := node.(*ast.BranchStmt)
			if !ok {
				return true
			}
			switch branch.Tok {
			case token.GOTO:
				hasEscape = true
				return false
			case token.CONTINUE:
				if branch.Label == nil || labels[pass.TypesInfo.ObjectOf(branch.Label)] == loop {
					hasEscape = true
					return false
				}
			}
			return true
		},
	)
	return hasEscape
}
