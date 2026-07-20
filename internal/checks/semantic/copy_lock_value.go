package semantic

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type copyLockValueCheck struct{}

func (copyLockValueCheck) Meta() Meta {
	return Meta{
		Code:            "copy-lock-value",
		Summary:         "detect copying values that contain sync.Mutex or sync.RWMutex",
		Explanation:     "Copying a struct after one of its locks has been used creates an independent copy of the lock state and breaks synchronization. Pass lock-containing values by pointer and avoid assigning, ranging over, or returning existing instances by value.",
		GoodExample:     "func update(state *State) { state.mu.Lock(); defer state.mu.Unlock() }",
		BadExample:      "func update(state State) { state.mu.Lock(); defer state.mu.Unlock() }",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (copyLockValueCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.AssignStmt)(nil),
			(*ast.CallExpr)(nil),
			(*ast.CompositeLit)(nil),
			(*ast.FuncDecl)(nil),
			(*ast.FuncLit)(nil),
			(*ast.RangeStmt)(nil),
			(*ast.SendStmt)(nil),
			(*ast.ValueSpec)(nil),
		},
		func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.FuncDecl:
				reportLockParameters(pass, node.Recv, "method receiver")
				reportLockParameters(pass, node.Type.Params, "function parameter")
			case *ast.FuncLit:
				reportLockParameters(pass, node.Type.Params, "function parameter")
			case *ast.AssignStmt:
				reportLockAssignments(pass, node.Lhs, node.Rhs, node)
			case *ast.ValueSpec:
				left := make([]ast.Expr, 0, len(node.Names))
				for _, name := range node.Names {
					left = append(left, name)
				}
				reportLockAssignments(pass, left, node.Values, node)
			case *ast.RangeStmt:
				reportLockRange(pass, node)
			case *ast.CallExpr:
				reportLockCall(pass, node)
			case *ast.CompositeLit:
				reportLockComposite(pass, node)
			case *ast.SendStmt:
				reportLockCopyExpression(pass, node.Value, node, "channel send")
			}
			return true
		},
	)
	forEachAnalysisFunction(
		pass,
		func(body *ast.BlockStmt, signature *types.Signature) {
			if body == nil || signature == nil {
				return
			}
			inspectFunctionBody(
				body,
				func(node ast.Node) bool {
					statement,
						ok := node.(*ast.ReturnStmt)
					if ok {
						reportLockReturn(pass, statement, signature)
					}
					return true
				},
			)
		},
	)
}

func reportLockComposite(pass *Pass, literal *ast.CompositeLit) {
	_, mapLiteral := types.Unalias(pass.TypesInfo.TypeOf(literal)).Underlying().(*types.Map)
	for _, element := range literal.Elts {
		expression, ok := element.(ast.Expr)
		if !ok {
			continue
		}
		if keyed, ok := expression.(*ast.KeyValueExpr); ok {
			if mapLiteral {
				reportLockCopyExpression(pass, keyed.Key, keyed.Key, "map key")
			}
			expression = keyed.Value
		}
		reportLockCopyExpression(pass, expression, expression, "composite literal")
	}
}

func reportLockCopyExpression(pass *Pass, expression ast.Expr, node ast.Node, role string) {
	lock := containedLockName(pass.TypesInfo.TypeOf(expression))
	if lock == "" || freshLockValue(pass, expression) {
		return
	}
	pass.Report(node, fmt.Sprintf("%s copies %s by value; it contains %s", role, analysisTypeName(pass.TypesInfo.TypeOf(expression)), lock))
}

func reportLockParameters(pass *Pass, fields *ast.FieldList, role string) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		valueType := pass.TypesInfo.TypeOf(field.Type)
		lock := containedLockName(valueType)
		if lock == "" {
			continue
		}
		node := ast.Node(field)
		if len(field.Names) != 0 {
			node = field.Names[0]
		}
		pass.Report(node, fmt.Sprintf("%s copies %s by value; it contains %s", role, analysisTypeName(valueType), lock))
	}
}

func reportLockAssignments(pass *Pass, left, right []ast.Expr, node ast.Node) {
	if len(left) != len(right) {
		return
	}
	for index, expression := range right {
		if identifier, ok := ast.Unparen(left[index]).(*ast.Ident); ok && identifier.Name == "_" {
			continue
		}
		lock := containedLockName(pass.TypesInfo.TypeOf(expression))
		if lock == "" || freshLockValue(pass, expression) || sameCopiedObject(pass, left[index], expression) {
			continue
		}
		pass.Report(node, fmt.Sprintf("assignment copies %s by value; it contains %s", analysisTypeName(pass.TypesInfo.TypeOf(expression)), lock))
	}
}

func reportLockRange(pass *Pass, statement *ast.RangeStmt) {
	if statement.Value == nil {
		return
	}
	if identifier, ok := statement.Value.(*ast.Ident); ok && identifier.Name == "_" {
		return
	}
	valueType := rangeValueType(pass.TypesInfo.TypeOf(statement.X))
	if valueType == nil {
		valueType = pass.TypesInfo.TypeOf(statement.Value)
	}
	lock := containedLockName(valueType)
	if lock == "" {
		return
	}
	pass.Report(statement.Value, fmt.Sprintf("range variable copies %s on each iteration; it contains %s", analysisTypeName(valueType), lock))
}

func reportLockCall(pass *Pass, call *ast.CallExpr) {
	if identifier, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		if builtin, ok := pass.TypesInfo.ObjectOf(identifier).(*types.Builtin); ok {
			if builtin.Name() == "append" {
				for _, argument := range call.Args[1:] {
					reportLockCopyExpression(pass, argument, argument, "append")
				}
			}
			return
		}
	}
	calleeType := pass.TypesInfo.TypeOf(call.Fun)
	if calleeType == nil {
		return
	}
	if _, callable := types.Unalias(calleeType).Underlying().(*types.Signature); !callable {
		return
	}
	for _, argument := range call.Args {
		lock := containedLockName(pass.TypesInfo.TypeOf(argument))
		if lock == "" || freshLockValue(pass, argument) {
			continue
		}
		pass.Report(argument, fmt.Sprintf("call copies %s by value; it contains %s", analysisTypeName(pass.TypesInfo.TypeOf(argument)), lock))
	}
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return
	}
	selection := pass.TypesInfo.Selections[selector]
	if selection == nil || selection.Kind() != types.MethodVal {
		return
	}
	method, _ := selection.Obj().(*types.Func)
	if method == nil {
		return
	}
	signature, _ := method.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil || freshLockValue(pass, selector.X) {
		return
	}
	lock := containedLockName(signature.Recv().Type())
	if lock == "" {
		return
	}
	pass.Report(selector, fmt.Sprintf("method call copies %s receiver by value; it contains %s", analysisTypeName(signature.Recv().Type()), lock))
}

func reportLockReturn(pass *Pass, statement *ast.ReturnStmt, signature *types.Signature) {
	if len(statement.Results) != signature.Results().Len() {
		return
	}
	for index, expression := range statement.Results {
		resultType := signature.Results().At(index).Type()
		lock := containedLockName(pass.TypesInfo.TypeOf(expression))
		if lock == "" {
			lock = containedLockName(resultType)
		}
		if lock == "" || freshLockValue(pass, expression) {
			continue
		}
		pass.Report(statement, fmt.Sprintf("return copies %s by value; it contains %s", analysisTypeName(resultType), lock))
	}
}

func containedLockName(valueType types.Type) string {
	return containedLockNameSeen(valueType, make(map[types.Type]bool))
}

func containedLockNameSeen(valueType types.Type, seen map[types.Type]bool) string {
	if valueType == nil {
		return ""
	}
	valueType = types.Unalias(valueType)
	if named, ok := valueType.(*types.Named); ok {
		if object := named.Obj(); object != nil && object.Pkg() != nil && object.Pkg().Path() == "sync" {
			switch object.Name() {
			case "Mutex", "RWMutex":
				return "sync." + object.Name()
			}
		}
	}
	if seen[valueType] {
		return ""
	}
	seen[valueType] = true
	switch underlying := valueType.Underlying().(type) {
	case *types.Array:
		return containedLockNameSeen(underlying.Elem(), seen)
	case *types.Struct:
		for index := range underlying.NumFields() {
			if lock := containedLockNameSeen(underlying.Field(index).Type(), seen); lock != "" {
				return lock
			}
		}
	}
	return ""
}

func freshLockValue(pass *Pass, expression ast.Expr) bool {
	switch expression := ast.Unparen(expression).(type) {
	case *ast.CompositeLit:
		return true
	case *ast.CallExpr:
		if pass.TypesInfo.Types[expression.Fun].IsType() {
			return false
		}
		return true
	case *ast.UnaryExpr:
		if expression.Op != token.MUL {
			return false
		}
		call, ok := ast.Unparen(expression.X).(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return false
		}
		identifier, ok := ast.Unparen(call.Fun).(*ast.Ident)
		if !ok {
			return false
		}
		builtin, _ := pass.TypesInfo.ObjectOf(identifier).(*types.Builtin)
		return builtin != nil && builtin.Name() == "new"
	default:
		return false
	}
}

func sameCopiedObject(pass *Pass, left, right ast.Expr) bool {
	leftIdentifier, leftOK := ast.Unparen(left).(*ast.Ident)
	rightIdentifier, rightOK := ast.Unparen(right).(*ast.Ident)
	return leftOK && rightOK && pass.TypesInfo.ObjectOf(leftIdentifier) == pass.TypesInfo.ObjectOf(rightIdentifier)
}

func rangeValueType(valueType types.Type) types.Type {
	if valueType == nil {
		return nil
	}
	switch underlying := types.Unalias(valueType).Underlying().(type) {
	case *types.Array:
		return underlying.Elem()
	case *types.Slice:
		return underlying.Elem()
	case *types.Map:
		return underlying.Elem()
	case *types.Chan:
		return underlying.Elem()
	default:
		return nil
	}
}

func analysisTypeName(valueType types.Type) string {
	if valueType == nil {
		return "lock-containing value"
	}
	return types.TypeString(valueType, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Name()
	})
}

func (copyLockValueCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
