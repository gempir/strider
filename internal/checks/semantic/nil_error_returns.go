package semantic

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type nilErrorReturnRule struct{}

func (nilErrorReturnRule) Meta() Meta {
	return Meta{
		Code:            "nil-error-return",
		Summary:         "detect nil errors returned from branches that prove an error is non-nil",
		Explanation:     "A branch entered because an error is non-nil should not return nil in the function's error result. Doing so discards the failure and makes the caller observe success.",
		GoodExample:     "if err != nil { return nil, err }",
		BadExample:      "if err != nil { return nil, nil }",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (nilErrorReturnRule) Run(pass *Pass) {
	forEachAnalysisFunction(
		pass,
		func(body *ast.BlockStmt, signature *types.Signature) {
			if body == nil || lastErrorResult(signature) < 0 {
				return
			}
			inspectFunctionBody(
				body,
				func(node ast.Node) bool {
					statement,
						ok := node.(*ast.IfStmt)
					if !ok {
						return true
					}
					if object := nonNilErrorComparison(pass, statement.Cond, token.NEQ); object != nil {
						reportNilErrorsInProvenBranch(pass, statement.Body, signature, object)
					}
					if object := nonNilErrorComparison(pass, statement.Cond, token.EQL); object != nil && statement.Else != nil {
						reportNilErrorsInProvenBranch(pass, statement.Else, signature, object)
					}
					return true
				},
			)
		},
	)
}

type nilValueWithNilErrorRule struct{}

func (nilValueWithNilErrorRule) Meta() Meta {
	return Meta{
		Code:            "nil-value-with-nil-error",
		Summary:         "detect nil payloads returned together with a nil error",
		Explanation:     "Returning an explicit nil payload and nil error leaves callers unable to distinguish a missing value from success. Return a descriptive error or a meaningful payload instead.",
		GoodExample:     "if value == nil { return nil, ErrNotFound }; return value, nil",
		BadExample:      "if value == nil { return nil, nil }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (nilValueWithNilErrorRule) Run(pass *Pass) {
	forEachAnalysisFunction(
		pass,
		func(body *ast.BlockStmt, signature *types.Signature) {
			errorIndex := lastErrorResult(signature)
			if body == nil || errorIndex < 0 || signature.Results().Len() < 2 {
				return
			}
			inspectFunctionBody(
				body,
				func(node ast.Node) bool {
					statement,
						ok := node.(*ast.ReturnStmt)
					if !ok || len(statement.Results) != signature.Results().Len() || !isExplicitNil(
						pass,
						statement.Results[errorIndex],
					) {
						return true
					}
					for index, expression := range statement.Results {
						if index == errorIndex || !isNilableType(
							signature.Results().At(index).Type(),
						) || !isExplicitNil(pass, expression) {
							continue
						}
						pass.Report(
							statement,
							"nil payload is returned with a nil error; return a meaningful value or a descriptive error",
						)
						break
					}
					return true
				},
			)
		},
	)
}

func forEachAnalysisFunction(pass *Pass, visit func(*ast.BlockStmt, *types.Signature)) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				switch function := node.(type) {
				case *ast.FuncDecl:
					object,
						_ := pass.TypesInfo.Defs[function.Name].(*types.Func)
					if object == nil {
						return true
					}
					signature,
						_ := object.Type().(*types.Signature)
					if signature != nil {
						visit(function.Body, signature)
					}
				case *ast.FuncLit:
					signature,
						_ := pass.TypesInfo.TypeOf(function.Type).(*types.Signature)
					if signature == nil {
						signature,
							_ = pass.TypesInfo.TypeOf(function).(*types.Signature)
					}
					if signature != nil {
						visit(function.Body, signature)
					}
				}
				return true
			},
		)
	}
}

func inspectFunctionBody(body *ast.BlockStmt, visit func(ast.Node) bool) {
	first := true
	ast.Inspect(
		body,
		func(node ast.Node) bool {
			if node == nil {
				return true
			}
			if _,
				nested := node.(*ast.FuncLit); nested && !first {
				return false
			}
			first = false
			return visit(node)
		},
	)
}

func nonNilErrorComparison(pass *Pass, expression ast.Expr, operator token.Token) types.Object {
	binary, ok := ast.Unparen(expression).(*ast.BinaryExpr)
	if !ok || binary.Op != operator {
		return nil
	}
	var candidate ast.Expr
	switch {
	case isExplicitNil(pass, binary.X):
		candidate = binary.Y
	case isExplicitNil(pass, binary.Y):
		candidate = binary.X
	default:
		return nil
	}
	identifier, ok := ast.Unparen(candidate).(*ast.Ident)
	if !ok || !isErrorType(pass.TypesInfo.TypeOf(identifier)) {
		return nil
	}
	return pass.TypesInfo.ObjectOf(identifier)
}

func reportNilErrorsInProvenBranch(
	pass *Pass,
	branch ast.Node,
	signature *types.Signature,
	provenError types.Object,
) {
	if branch == nil {
		return
	}
	inspectFunctionBodyNode(
		branch,
		func(node ast.Node) bool {
			statement,
				ok := node.(*ast.ReturnStmt)
			if !ok || len(statement.Results) != signature.Results().Len() {
				return true
			}
			if branchAssignsObjectBeforeReturn(pass, branch, provenError, statement) {
				return true
			}
			for index := range signature.Results().Len() {
				if !isErrorType(signature.Results().At(index).Type()) || !isExplicitNil(
					pass,
					statement.Results[index],
				) {
					continue
				}
				pass.Report(
					statement,
					"this branch proves an error is non-nil but returns nil in an error result",
				)
				break
			}
			return true
		},
	)
}

func inspectFunctionBodyNode(root ast.Node, visit func(ast.Node) bool) {
	first := true
	ast.Inspect(
		root,
		func(node ast.Node) bool {
			if node == nil {
				return true
			}
			if _,
				nested := node.(*ast.FuncLit); nested && !first {
				return false
			}
			first = false
			return visit(node)
		},
	)
}

func branchAssignsObjectBeforeReturn(
	pass *Pass,
	branch ast.Node,
	object types.Object,
	returning *ast.ReturnStmt,
) bool {
	block, ok := branch.(*ast.BlockStmt)
	if !ok {
		if statement, statementOK := branch.(ast.Stmt); statementOK {
			return statementAssignsObjectBeforeReturn(pass, statement, object, returning)
		}
		return false
	}
	return blockAssignsObjectBeforeReturn(pass, block, object, returning)
}

func blockAssignsObjectBeforeReturn(
	pass *Pass,
	block *ast.BlockStmt,
	object types.Object,
	returning *ast.ReturnStmt,
) bool {
	for _, statement := range block.List {
		if statement.Pos() <= returning.Pos() && returning.End() <= statement.End() {
			return statementAssignsObjectBeforeReturn(pass, statement, object, returning)
		}
		if statement.End() <= returning.Pos() && directStatementAssignsObject(
			pass,
			statement,
			object,
		) {
			return true
		}
	}
	return false
}

func statementAssignsObjectBeforeReturn(
	pass *Pass,
	statement ast.Stmt,
	object types.Object,
	returning *ast.ReturnStmt,
) bool {
	switch statement := statement.(type) {
	case *ast.BlockStmt:
		return blockAssignsObjectBeforeReturn(pass, statement, object, returning)
	case *ast.IfStmt:
		if statement.Init != nil && directStatementAssignsObject(pass, statement.Init, object) {
			return true
		}
		if statement.Body.Pos() <= returning.Pos() && returning.End() <= statement.Body.End() {
			return blockAssignsObjectBeforeReturn(pass, statement.Body, object, returning)
		}
		if statement.Else != nil && statement.Else.Pos() <= returning.Pos() && returning.End() <= statement.Else.End() {
			return statementAssignsObjectBeforeReturn(pass, statement.Else, object, returning)
		}
	}
	return false
}

func directStatementAssignsObject(pass *Pass, statement ast.Stmt, object types.Object) bool {
	assignment, ok := statement.(*ast.AssignStmt)
	if !ok {
		return false
	}
	for _, expression := range assignment.Lhs {
		identifier, ok := ast.Unparen(expression).(*ast.Ident)
		if ok && pass.TypesInfo.ObjectOf(identifier) == object {
			return true
		}
	}
	return false
}

func lastErrorResult(signature *types.Signature) int {
	if signature == nil || signature.Results() == nil || signature.Results().Len() == 0 {
		return -1
	}
	index := signature.Results().Len() - 1
	if !isErrorType(signature.Results().At(index).Type()) {
		return -1
	}
	return index
}

func isErrorType(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	errorType := types.Universe.Lookup("error").Type()
	return types.AssignableTo(valueType, errorType)
}

func isExplicitNil(pass *Pass, expression ast.Expr) bool {
	identifier, ok := ast.Unparen(expression).(*ast.Ident)
	if !ok || identifier.Name != "nil" {
		return false
	}
	_, ok = pass.TypesInfo.ObjectOf(identifier).(*types.Nil)
	return ok
}

func isNilableType(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	switch types.Unalias(valueType).Underlying().(type) {
	case *types.Chan, *types.Signature, *types.Interface, *types.Map, *types.Pointer, *types.Slice:
		return true
	default:
		return false
	}
}
