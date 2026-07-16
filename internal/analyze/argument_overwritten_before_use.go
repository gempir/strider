package analyze

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
	"golang.org/x/tools/go/ssa"
)

type argumentOverwrittenBeforeUseRule struct{}

func (argumentOverwrittenBeforeUseRule) Meta() Meta {
	return Meta{
		Code:            "argument-overwritten-before-use",
		Summary:         "detect function arguments replaced before their incoming value is used",
		Explanation:     "Overwriting a function argument before reading its incoming value makes that input meaningless. The assignment may be accidental, or the argument may no longer belong in the function signature.",
		GoodExample:     "func normalize(value string) string { use(value); value = fallback; return value }",
		BadExample:      "func normalize(value string) string { value = fallback; return value }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (argumentOverwrittenBeforeUseRule) Run(pass *Pass) {
	for _, function := range pass.Functions {
		if function == nil || function.Synthetic != "" || function.Blocks == nil {
			continue
		}
		functionType, body, ok := functionSyntaxParts(function.Syntax())
		if !ok || functionType.Params == nil {
			continue
		}
		for _, field := range functionType.Params.List {
			for _, argument := range field.Names {
				if argument.Name == "_" {
					continue
				}
				object := pass.TypesInfo.ObjectOf(argument)
				parameter := ssaParameterForObject(function, object)
				if parameter == nil || hasNonDebugReferrer(parameter) {
					continue
				}
				assignment := firstAssignmentToObject(pass, body, object)
				if assignment == nil {
					continue
				}
				pass.Report(
					argument,
					fmt.Sprintf("argument %s is overwritten before its incoming value is used", argument.Name),
				)
			}
		}
	}
}

func functionSyntaxParts(node ast.Node) (*ast.FuncType, *ast.BlockStmt, bool) {
	switch node := node.(type) {
	case *ast.FuncDecl:
		return node.Type, node.Body, node.Body != nil
	case *ast.FuncLit:
		return node.Type, node.Body, node.Body != nil
	default:
		return nil, nil, false
	}
}

func ssaParameterForObject(function *ssa.Function, object types.Object) *ssa.Parameter {
	if object == nil {
		return nil
	}
	for _, parameter := range function.Params {
		if parameter.Object() == object {
			return parameter
		}
	}
	return nil
}

func hasNonDebugReferrer(value ssa.Value) bool {
	references := value.Referrers()
	if references == nil {
		return false
	}
	for _, reference := range *references {
		if _, debug := reference.(*ssa.DebugRef); !debug {
			return true
		}
	}
	return false
}

func firstAssignmentToObject(pass *Pass, body *ast.BlockStmt, object types.Object) ast.Node {
	var found ast.Node
	inspectWithoutClosures(body, func(node ast.Node) bool {
		if found != nil {
			return false
		}
		assignment, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, left := range assignment.Lhs {
			identifier, ok := left.(*ast.Ident)
			if ok && pass.TypesInfo.ObjectOf(identifier) == object {
				found = assignment
				return false
			}
		}
		return true
	})
	return found
}
