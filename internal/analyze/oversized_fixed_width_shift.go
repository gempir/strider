package analyze

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type oversizedFixedWidthShiftRule struct {}

func (oversizedFixedWidthShiftRule) Meta() Meta {
	return Meta{
		Code: "oversized-fixed-width-shift",
		Summary: "detect shifts that always clear fixed-width integers",
		Explanation: "Shifting a fixed-width integer by its full width or more always clears every value bit. This is usually an incorrect shift count. Machine-sized int, uint, and uintptr are excluded because width-dependent bit manipulation can be intentional.",
		GoodExample: "value := uint8(1) << 7",
		BadExample: "value := uint8(1) << 8",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (oversizedFixedWidthShiftRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				switch expression := node.(type) {
				case *ast.BinaryExpr:
					if expression.Op == token.SHL || expression.Op == token.SHR {
						reportOversizedShift(pass, expression, expression.X, expression.Y)
					}
				case *ast.AssignStmt:
					if(expression.Tok == token.SHL_ASSIGN || expression.Tok == token.SHR_ASSIGN) && len(
						expression.Lhs,
					) == 1 && len(expression.Rhs) == 1 {
						reportOversizedShift(pass, expression, expression.Lhs[0], expression.Rhs[0])
					}
				}
				return true
			},
		)
	}
}

func reportOversizedShift(pass *Pass, node ast.Node, value, count ast.Expr) {
	basic, ok := pass.TypesInfo.TypeOf(value).Underlying().(*types.Basic)
	if !ok || !fixedWidthInteger(basic.Kind()) {
		return
	}
	constantValue := pass.TypesInfo.Types[count].Value
	if constantValue == nil || constantValue.Kind() != constant.Int {
		return
	}
	shift, ok := constant.Int64Val(constantValue)
	if !ok || shift < 0 {
		return
	}
	width := pass.TypesSizes.Sizeof(basic) * 8
	if shift < width {
		return
	}
	pass.Report(
		node,
		fmt.Sprintf("shifting a %d-bit value by %d bits always clears it", width, shift),
	)
}

func fixedWidthInteger(kind types.BasicKind) bool {
	switch kind {
	case types.Int8, types.Int16, types.Int32, types.Int64, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
		return true
	default:
		return false
	}
}
