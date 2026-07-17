package analyze

import (
	"fmt"
	"go/constant"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type invalidStrconvArgumentRule struct {}

type strconvConstraint struct {
	index int
	validate func(int64) string
}

func (invalidStrconvArgumentRule) Meta() Meta {
	return Meta{
		Code: "invalid-strconv-argument",
		Summary: "detect invalid constant arguments to strconv functions",
		Explanation: "strconv parsing and formatting functions accept only documented number bases, bit sizes, and floating-point format characters. Invalid constants always return errors or produce unusable results.",
		GoodExample: `strconv.ParseInt(value, 10, 64)`,
		BadExample: `strconv.ParseInt(value, 1, 128)`,
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (invalidStrconvArgumentRule) Run(pass *Pass) {
	calls := argumentsByCallPosition(pass.Files)
	for _, function := range pass.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok {
					continue
				}
				constraints := strconvConstraints(call)
				for _, constraint := range constraints {
					if len(call.Common().Args) <= constraint.index {
						continue
					}
					value, ok := ssaInt64(call.Common().Args[constraint.index])
					if !ok {
						continue
					}
					message := constraint.validate(value)
					if message == "" {
						continue
					}
					node := explicitCallArgument(calls[call.Pos()], constraint.index, call.Pos())
					pass.Report(node, message)
				}
			}
		}
	}
}

func strconvConstraints(call ssa.CallInstruction) []strconvConstraint {
	callee := call.Common().StaticCallee()
	if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil || callee.Object().Pkg().Path() != "strconv" {
		return nil
	}
	switch callee.Object().Name() {
	case "ParseComplex":
		return[]strconvConstraint{{1, validateComplexBitSize}}
	case "ParseFloat":
		return[]strconvConstraint{{1, validateFloatBitSize}}
	case "ParseInt", "ParseUint":
		return[]strconvConstraint{{1, validateParsingBase}, {2, validateIntegerBitSize}}
	case "FormatComplex":
		return[]strconvConstraint{{1, validateFloatFormat}, {3, validateComplexBitSize}}
	case "FormatFloat":
		return[]strconvConstraint{{1, validateFloatFormat}, {3, validateFloatBitSize}}
	case "FormatInt", "FormatUint":
		return[]strconvConstraint{{1, validateFormattingBase}}
	case "AppendFloat":
		return[]strconvConstraint{{2, validateFloatFormat}, {4, validateFloatBitSize}}
	case "AppendInt", "AppendUint":
		return[]strconvConstraint{{2, validateFormattingBase}}
	default:
		return nil
	}
}

func ssaInt64(value ssa.Value) (int64, bool) {
	constantValue := ssaConstant(value)
	if constantValue == nil || constantValue.Value == nil || constantValue.Value.Kind() != constant.Int {
		return 0, false
	}
	return constant.Int64Val(constantValue.Value)
}

func validateComplexBitSize(value int64) string {
	if value != 64 && value != 128 {
		return "bit size must be either 64 or 128 for complex numbers"
	}
	return ""
}

func validateFloatBitSize(value int64) string {
	if value != 32 && value != 64 {
		return "bit size must be either 32 or 64 for floating-point numbers"
	}
	return ""
}

func validateIntegerBitSize(value int64) string {
	if value < 0 || value > 64 {
		return "integer bit size must be between 0 and 64"
	}
	return ""
}

func validateParsingBase(value int64) string {
	if value != 0 && (value < 2 || value > 36) {
		return "parsing base must be 0 or between 2 and 36"
	}
	return ""
}

func validateFormattingBase(value int64) string {
	if value < 2 || value > 36 {
		return "formatting base must be between 2 and 36"
	}
	return ""
}

func validateFloatFormat(value int64) string {
	switch rune(value) {
	case 'b', 'e', 'E', 'f', 'g', 'G', 'x', 'X':
		return ""
	default:
		return fmt.Sprintf("unknown floating-point format %q", rune(value))
	}
}
