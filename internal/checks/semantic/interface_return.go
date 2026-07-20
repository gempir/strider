package semantic

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gempir/strider/internal/diagnostic"
)

type constructorInterfaceReturnRule struct{}

func (constructorInterfaceReturnRule) Meta() Meta {
	return Meta{
		Code:            "constructor-interface-return",
		Summary:         "detect constructors that hide a single concrete result",
		Explanation:     "Callers can define the small interface they need when constructors return concrete types. This conservative check only reports exported New-style constructors whose non-nil returns consistently reveal one local concrete implementation of a non-empty local interface; error, any, standard-library interfaces, and polymorphic results are ignored.",
		GoodExample:     "func NewStore() *memoryStore { return &memoryStore{} }",
		BadExample:      "func NewStore() Store { return &memoryStore{} }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (constructorInterfaceReturnRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil || function.Recv != nil || !constructorLikeName(function.Name.Name) {
				continue
			}
			object, ok := pass.TypesInfo.Defs[function.Name].(*types.Func)
			if !ok {
				continue
			}
			signature, ok := object.Type().(*types.Signature)
			if !ok {
				continue
			}
			for resultIndex := 0; resultIndex < signature.Results().Len(); resultIndex++ {
				resultType := signature.Results().At(resultIndex).Type()
				if !localNontrivialInterface(pass, resultType) {
					continue
				}
				concrete, ok := consistentConcreteReturns(pass, function.Body, signature, resultIndex, resultType)
				if !ok {
					continue
				}
				pass.Report(
					function.Name,
					fmt.Sprintf(
						"%s returns interface %s although its concrete result is consistently %s; return the concrete type",
						function.Name.Name,
						types.TypeString(resultType, performanceTypeQualifier(pass.Types)),
						types.TypeString(concrete, performanceTypeQualifier(pass.Types)),
					),
				)
			}
		}
	}
}

func constructorLikeName(name string) bool {
	if name == "New" {
		return true
	}
	if !strings.HasPrefix(name, "New") || len(name) == len("New") {
		return false
	}
	next, _ := utf8.DecodeRuneInString(name[len("New"):])
	return unicode.IsUpper(next) || unicode.IsDigit(next)
}

func localNontrivialInterface(pass *Pass, valueType types.Type) bool {
	unaliased := types.Unalias(valueType)
	named, ok := unaliased.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() != pass.Types {
		return false
	}
	interfaceType, ok := named.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	interfaceType.Complete()
	return interfaceType.NumMethods() != 0
}

func consistentConcreteReturns(pass *Pass, body *ast.BlockStmt, signature *types.Signature, resultIndex int, interfaceType types.Type) (types.Type, bool) {
	var concrete types.Type
	valid := true
	found := false
	ast.Inspect(
		body,
		func(node ast.Node) bool {
			if !valid {
				return false
			}
			if _,
				nested := node.(*ast.FuncLit); nested {
				return false
			}
			statement,
				ok := node.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			if len(statement.Results) != signature.Results().Len() {
				valid = false
				return false
			}
			expression := unwrapInterfaceResultConversion(pass, statement.Results[resultIndex], interfaceType)
			if isNilExpression(pass, expression) {
				return true
			}
			returnedType := pass.TypesInfo.TypeOf(expression)
			if !localConcreteType(pass, returnedType) || !types.AssignableTo(returnedType, interfaceType) {
				valid = false
				return false
			}
			if !found {
				concrete = returnedType
				found = true
				return true
			}
			if !types.Identical(concrete, returnedType) {
				valid = false
				return false
			}
			return true
		},
	)
	return concrete, valid && found
}

func unwrapInterfaceResultConversion(pass *Pass, expression ast.Expr, resultType types.Type) ast.Expr {
	for {
		switch current := expression.(type) {
		case *ast.ParenExpr:
			expression = current.X
		case *ast.CallExpr:
			if len(current.Args) != 1 || !pass.TypesInfo.Types[current.Fun].IsType() || !types.Identical(pass.TypesInfo.TypeOf(current), resultType) {
				return expression
			}
			expression = current.Args[0]
		default:
			return expression
		}
	}
}

func isNilExpression(pass *Pass, expression ast.Expr) bool {
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return false
	}
	_, ok = pass.TypesInfo.ObjectOf(identifier).(*types.Nil)
	return ok
}

func localConcreteType(pass *Pass, valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	unaliased := types.Unalias(valueType)
	if pointer, ok := unaliased.(*types.Pointer); ok {
		unaliased = types.Unalias(pointer.Elem())
	}
	named, ok := unaliased.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() != pass.Types {
		return false
	}
	_, isInterface := named.Underlying().(*types.Interface)
	return !isInterface
}

func performanceTypeQualifier(current *types.Package) types.Qualifier {
	return func(pkg *types.Package) string {
		if pkg == nil || pkg == current {
			return ""
		}
		return pkg.Name()
	}
}

func (constructorInterfaceReturnRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
