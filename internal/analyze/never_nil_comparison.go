package analyze

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
	"golang.org/x/tools/go/ssa"
)

type neverNilComparisonRule struct{}

func (neverNilComparisonRule) Meta() Meta {
	return Meta{
		Code:            "never-nil-comparison",
		Summary:         "detect nil checks on values proven to be non-nil",
		Explanation:     "Fresh allocations, make results, functions, closures, and values flowing exclusively from those sources cannot be nil. Comparing them with nil has a fixed result and often means the wrong value was checked.",
		GoodExample:     "var values []int; if values == nil { initialize() }",
		BadExample:      "values := make([]int, 0); if values == nil { unreachable() }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (neverNilComparisonRule) Run(pass *Pass) {
	for _, function := range pass.Functions {
		if function == nil || function.Synthetic != "" || function.Blocks == nil ||
			function.Syntax() == nil {
			continue
		}
		inspectFunctionSyntax(function.Syntax(), func(node ast.Node) bool {
			ifStatement, ok := node.(*ast.IfStmt)
			if !ok {
				return true
			}
			binary, ok := ast.Unparen(ifStatement.Cond).(*ast.BinaryExpr)
			if !ok || binary.Op != token.EQL && binary.Op != token.NEQ {
				return true
			}
			checked, ok := nilCheckedExpression(pass, binary.X, binary.Y)
			if !ok {
				checked, ok = nilCheckedExpression(pass, binary.Y, binary.X)
			}
			if !ok {
				return true
			}
			value, isAddress := function.ValueForExpr(checked)
			if value == nil || isAddress || !ssaValueNeverNil(value, make(map[ssa.Value]bool)) {
				return true
			}
			truth := "never"
			if binary.Op == token.NEQ {
				truth = "always"
			}
			message := "this nil comparison is " + truth + " true"
			if _, functionValue := flattenEquivalentPhi(value).(*ssa.Function); functionValue {
				message = "function values are never nil; did you mean to call the function?"
			}
			pass.Report(binary, message)
			return true
		})
	}
}

func nilCheckedExpression(pass *Pass, value, nilExpression ast.Expr) (ast.Expr, bool) {
	identifier, ok := ast.Unparen(nilExpression).(*ast.Ident)
	if !ok {
		return nil, false
	}
	if _, isNil := pass.TypesInfo.ObjectOf(identifier).(*types.Nil); !isNil {
		return nil, false
	}
	if address, ok := ast.Unparen(value).(*ast.UnaryExpr); ok && address.Op == token.AND {
		return nil, false
	}
	return value, true
}

func ssaValueNeverNil(value ssa.Value, seen map[ssa.Value]bool) bool {
	if value == nil {
		return false
	}
	if seen[value] {
		return true
	}
	seen[value] = true
	switch value := value.(type) {
	case *ssa.MakeClosure, *ssa.Function, *ssa.MakeChan, *ssa.MakeMap, *ssa.MakeSlice,
		*ssa.Alloc:
		return true
	case *ssa.Slice:
		return ssaValueNeverNil(value.X, seen)
	case *ssa.FieldAddr:
		return ssaValueNeverNil(value.X, seen)
	case *ssa.IndexAddr:
		return ssaValueNeverNil(value.X, seen)
	case *ssa.ChangeType:
		return ssaValueNeverNil(value.X, seen)
	case *ssa.Convert:
		return ssaValueNeverNil(value.X, seen)
	case *ssa.Phi:
		if len(value.Edges) == 0 {
			return false
		}
		for _, edge := range value.Edges {
			if !ssaValueNeverNil(edge, seen) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
