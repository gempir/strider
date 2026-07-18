package semantic

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type appendToSizedSliceRule struct{}

func (appendToSizedSliceRule) Meta() Meta {
	return Meta{
		Code:            "append-to-sized-slice",
		Summary:         "detect appends to slices created with a known positive length",
		Explanation:     "make([]T, n) with a compile-time positive n creates existing zero-valued elements, not capacity for future elements. Appending places new values after those zeros. When the intent is to grow the slice, create it with length zero and capacity n, or reset its length before appending.",
		GoodExample:     "values := make([]int, 0, count)\nvalues = append(values, next)",
		BadExample:      "values := make([]int, count)\nvalues = append(values, next)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (appendToSizedSliceRule) Run(pass *Pass) {
	eligible := localPositiveLengthMakes(pass)
	reported := make(map[token.Pos]bool)
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(*ssa.Call)
				if !ok || !isAppendBuiltin(call.Common()) || len(call.Common().Args) == 0 {
					continue
				}
				origin := sizedSliceOrigin(call.Common().Args[0], make(map[ssa.Value]bool))
				if origin == nil || reported[origin.Pos()] {
					continue
				}
				candidate, ok := eligible[origin.Pos()]
				if !ok {
					continue
				}
				reported[origin.Pos()] = true
				pass.Report(
					positionNode{position: call.Pos()},
					fmt.Sprintf(
						"%s already has a positive length; use make with length zero and capacity, or reslice to zero before append",
						candidate.name,
					),
				)
			}
		}
	}
}

type sizedSliceCandidate struct {
	name string
}

func localPositiveLengthMakes(pass *Pass) map[token.Pos]sizedSliceCandidate {
	result := make(map[token.Pos]sizedSliceCandidate)
	visitBody := func(body *ast.BlockStmt) {
		ast.Inspect(
			body,
			func(node ast.Node) bool {
				switch node := node.(type) {
				case *ast.FuncLit:
					return false
				case *ast.AssignStmt:
					for index, right := range node.Rhs {
						if index >= len(node.Lhs) {
							break
						}
						identifier,
							ok := node.Lhs[index].(*ast.Ident)
						if !ok {
							continue
						}
						addPositiveLengthMake(pass, result, identifier, right)
					}
				case *ast.DeclStmt:
					declaration,
						ok := node.Decl.(*ast.GenDecl)
					if !ok || declaration.Tok != token.VAR {
						return true
					}
					for _, specification := range declaration.Specs {
						value,
							ok := specification.(*ast.ValueSpec)
						if !ok {
							continue
						}
						for index, right := range value.Values {
							if index >= len(value.Names) {
								break
							}
							addPositiveLengthMake(pass, result, value.Names[index], right)
						}
					}
				}
				return true
			},
		)
	}
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				switch node := node.(type) {
				case *ast.FuncDecl:
					if node.Body != nil {
						visitBody(node.Body)
					}
				case *ast.FuncLit:
					visitBody(node.Body)
				}
				return true
			},
		)
	}
	return result
}

func addPositiveLengthMake(
	pass *Pass,
	result map[token.Pos]sizedSliceCandidate,
	identifier *ast.Ident,
	expression ast.Expr,
) {
	variable, ok := pass.TypesInfo.ObjectOf(identifier).(*types.Var)
	if !ok || variable.IsField() {
		return
	}
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) < 2 || !isBuiltin(pass.TypesInfo, call.Fun, "make") {
		return
	}
	if _, ok := types.Unalias(pass.TypesInfo.TypeOf(call)).Underlying().(*types.Slice); !ok {
		return
	}
	length := pass.TypesInfo.Types[call.Args[1]].Value
	if length == nil || length.Kind() != constant.Int || constant.Sign(length) <= 0 {
		return
	}
	candidate := sizedSliceCandidate{name: identifier.Name}
	result[call.Pos()] = candidate
	result[call.Lparen] = candidate
}

func isBuiltin(info *types.Info, expression ast.Expr, name string) bool {
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := info.Uses[identifier].(*types.Builtin)
	return ok && builtin.Name() == name
}

func isAppendBuiltin(call *ssa.CallCommon) bool {
	builtin, ok := call.Value.(*ssa.Builtin)
	return ok && builtin.Name() == "append"
}

func sizedSliceOrigin(value ssa.Value, visiting map[ssa.Value]bool) ssa.Value {
	return traceSizedSliceOrigin(value, visiting).origin
}

type sizedSliceTrace struct {
	origin ssa.Value
	cycle  bool
	valid  bool
}

func traceSizedSliceOrigin(value ssa.Value, visiting map[ssa.Value]bool) sizedSliceTrace {
	if value == nil || visiting[value] {
		return sizedSliceTrace{cycle: value != nil, valid: value != nil}
	}
	visiting[value] = true
	defer delete(visiting, value)

	switch value := value.(type) {
	case *ssa.MakeSlice:
		length := ssaConstant(value.Len)
		if length == nil || length.Value == nil || length.Value.Kind() != constant.Int || constant.Sign(
			length.Value,
		) <= 0 {
			return sizedSliceTrace{}
		}
		return sizedSliceTrace{origin: value, valid: true}
	case *ssa.ChangeType:
		return traceSizedSliceOrigin(value.X, visiting)
	case *ssa.Phi:
		var common ssa.Value
		cycle := false
		for _, edge := range value.Edges {
			trace := traceSizedSliceOrigin(edge, visiting)
			if !trace.valid {
				return sizedSliceTrace{}
			}
			cycle = cycle || trace.cycle
			if trace.origin == nil {
				continue
			}
			if common == nil {
				common = trace.origin
			} else if common != trace.origin {
				return sizedSliceTrace{}
			}
		}
		return sizedSliceTrace{origin: common, cycle: cycle, valid: common != nil || cycle}
	case *ssa.Slice:
		if allocation, ok := value.X.(*ssa.Alloc); ok && allocation.Comment == "makeslice" && value.Low == nil && value.Max == nil {
			high := ssaConstant(value.High)
			if high != nil && high.Value != nil && high.Value.Kind() == constant.Int && constant.Sign(
				high.Value,
			) > 0 {
				return sizedSliceTrace{origin: allocation, valid: true}
			}
		}
		// A full slice expression preserves the original non-zero length. Any
		// explicit bound may intentionally reset or otherwise adjust the slice.
		if value.Low != nil || value.High != nil || value.Max != nil {
			return sizedSliceTrace{}
		}
		return traceSizedSliceOrigin(value.X, visiting)
	case *ssa.Call:
		if !isAppendBuiltin(value.Common()) || len(value.Common().Args) == 0 {
			return sizedSliceTrace{}
		}
		return traceSizedSliceOrigin(value.Common().Args[0], visiting)
	default:
		return sizedSliceTrace{}
	}
}
