package semantic

import (
	"go/ast"

	"github.com/gempir/strider/internal/diagnostic"
)

type contextStoredInStructRule struct {}

func (contextStoredInStructRule) Meta() Meta {
	return Meta{
		Code: "context-stored-in-struct",
		Summary: "detect context.Context fields in structs",
		Explanation: "Contexts carry request-scoped cancellation, deadlines, and values. Storing one in a struct obscures its lifetime and can reuse stale request state; pass it explicitly to each operation that needs it.",
		GoodExample: "func (service *Service) Run(ctx context.Context) error",
		BadExample: "type Service struct { ctx context.Context }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (contextStoredInStructRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				structure,
				ok := node.(*ast.StructType)
				if !ok || structure.Fields == nil {
					return true
				}
				for _,
				field := range structure.Fields.List {
					if !isContextType(pass.TypesInfo.TypeOf(field.Type)) {
						continue
					}
					pass.Report(field, "do not store context.Context in a struct; pass it explicitly to each operation")
				}
				return true
			},
		)
	}
}
