package semantic

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type oddPairedArgumentsRule struct{}

func (oddPairedArgumentsRule) Meta() Meta {
	return Meta{
		Code:            "odd-paired-arguments",
		Summary:         "detect odd element counts passed to pair-oriented APIs",
		Explanation:     "Some functions consume slice or variadic elements in pairs and reject odd lengths. The analyzer recognizes standard pair-oriented APIs and local functions that enforce an even length by panicking, then checks calls whose argument length is statically known.",
		GoodExample:     `strings.NewReplacer("old", "new")`,
		BadExample:      `strings.NewReplacer("old", "new", "orphan")`,
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (oddPairedArgumentsRule) Run(pass *Pass) {
	contracts := pairedArgumentContracts(pass)
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
		},
		func(node ast.Node) bool {
			call,
				ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			function := calledFunction(pass.TypesInfo, call.Fun)
			if function == nil {
				return true
			}
			parameterIndex,
				known := contracts[function]
			if function.Pkg() != nil && function.Pkg().Path() == "strings" && function.Name() == "NewReplacer" {
				parameterIndex,
					known = 0,
					true
			}
			if !known {
				return true
			}
			length,
				argument := pairedCallLength(pass, call, function, parameterIndex)
			if length < 0 || length%2 == 0 {
				return true
			}
			pass.Report(argument, fmt.Sprintf("paired argument requires an even number of elements, but this call provides %d", length))
			return true
		},
	)
}

func pairedArgumentContracts(pass *Pass) map[*types.Func]int {
	contracts := make(map[*types.Func]int)
	for _, file := range pass.Files {
		for _, declaration := range file.Decls {
			functionDeclaration, ok := declaration.(*ast.FuncDecl)
			if !ok || functionDeclaration.Body == nil {
				continue
			}
			function, _ := pass.TypesInfo.Defs[functionDeclaration.Name].(*types.Func)
			if function == nil {
				continue
			}
			ast.Inspect(
				functionDeclaration.Body,
				func(node ast.Node) bool {
					conditional,
						ok := node.(*ast.IfStmt)
					if !ok || !blockImmediatelyPanics(pass, conditional.Body) {
						return true
					}
					parameter := oddLengthParameter(pass, conditional.Cond)
					if parameter == nil {
						return true
					}
					signature,
						_ := function.Type().(*types.Signature)
					if signature == nil {
						return true
					}
					for index := range signature.Params().Len() {
						if signature.Params().At(index) == parameter {
							contracts[function] = index
							return false
						}
					}
					return true
				},
			)
		}
	}
	return contracts
}

func blockImmediatelyPanics(pass *Pass, block *ast.BlockStmt) bool {
	if block == nil || len(block.List) == 0 {
		return false
	}
	expression, ok := block.List[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := expression.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	identifier, ok := call.Fun.(*ast.Ident)
	if !ok || identifier.Name != "panic" {
		return false
	}
	builtin, _ := pass.TypesInfo.Uses[identifier].(*types.Builtin)
	return builtin != nil
}

func oddLengthParameter(pass *Pass, expression ast.Expr) *types.Var {
	comparison, ok := expression.(*ast.BinaryExpr)
	if !ok || (comparison.Op != token.NEQ && comparison.Op != token.EQL) {
		return nil
	}
	needle := int64(0)
	if comparison.Op == token.EQL {
		needle = 1
	}
	var remainder ast.Expr
	switch {
	case constantInt(pass, comparison.Y) == needle:
		remainder = comparison.X
	case constantInt(pass, comparison.X) == needle:
		remainder = comparison.Y
	default:
		return nil
	}
	modulo, ok := remainder.(*ast.BinaryExpr)
	if !ok || modulo.Op != token.REM || constantInt(pass, modulo.Y) != 2 {
		return nil
	}
	lengthCall, ok := modulo.X.(*ast.CallExpr)
	if !ok || len(lengthCall.Args) != 1 {
		return nil
	}
	lengthBuiltin, ok := lengthCall.Fun.(*ast.Ident)
	if !ok || lengthBuiltin.Name != "len" {
		return nil
	}
	if _, ok := pass.TypesInfo.Uses[lengthBuiltin].(*types.Builtin); !ok {
		return nil
	}
	parameterIdentifier, ok := lengthCall.Args[0].(*ast.Ident)
	if !ok {
		return nil
	}
	parameter, _ := pass.TypesInfo.Uses[parameterIdentifier].(*types.Var)
	return parameter
}

func constantInt(pass *Pass, expression ast.Expr) int64 {
	value := pass.TypesInfo.Types[expression].Value
	if value == nil || value.Kind() != constant.Int {
		return -1
	}
	integer, ok := constant.Int64Val(value)
	if !ok {
		return -1
	}
	return integer
}

func pairedCallLength(pass *Pass, call *ast.CallExpr, function *types.Func, parameterIndex int) (int, ast.Node) {
	signature, _ := function.Type().(*types.Signature)
	if signature == nil || parameterIndex >= signature.Params().Len() {
		return -1, call
	}
	if signature.Variadic() && parameterIndex == signature.Params().Len()-1 {
		if call.Ellipsis.IsValid() {
			if len(call.Args) == 0 {
				return -1, call
			}
			return knownCompositeLength(pass, call.Args[len(call.Args)-1]), call.Args[len(call.Args)-1]
		}
		if parameterIndex > len(call.Args) {
			return -1, call
		}
		argument := ast.Node(call)
		if parameterIndex < len(call.Args) {
			argument = call.Args[parameterIndex]
		}
		return len(call.Args) - parameterIndex, argument
	}
	if parameterIndex >= len(call.Args) {
		return -1, call
	}
	return knownCompositeLength(pass, call.Args[parameterIndex]), call.Args[parameterIndex]
}

func knownCompositeLength(pass *Pass, expression ast.Expr) int {
	composite, ok := expression.(*ast.CompositeLit)
	if !ok {
		return -1
	}
	if _, ok := pass.TypesInfo.TypeOf(composite).Underlying().(*types.Slice); !ok {
		return -1
	}
	for _, element := range composite.Elts {
		if _, keyed := element.(*ast.KeyValueExpr); keyed {
			return -1
		}
	}
	return len(composite.Elts)
}

func (oddPairedArgumentsRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
