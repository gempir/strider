package semantic

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

type singleArgumentAppendRule struct{}

func (singleArgumentAppendRule) Meta() Meta {
	return Meta{
		Code:            "single-argument-append",
		Summary:         "detect append calls that add no elements",
		Explanation:     "Calling the predeclared append function with only a slice argument returns that same slice unchanged. Assign the slice directly instead.",
		GoodExample:     "destination = source",
		BadExample:      "destination = append(source)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (singleArgumentAppendRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				call,
					ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) != 1 || call.Ellipsis.IsValid() {
					return true
				}
				identifier,
					ok := call.Fun.(*ast.Ident)
				if !ok {
					return true
				}
				builtin,
					ok := pass.TypesInfo.ObjectOf(identifier).(*types.Builtin)
				if !ok || builtin.Name() != "append" {
					return true
				}
				message := "append with no elements returns the original slice unchanged"
				if hasCommentBetween(file.Comments, identifier.End(), call.Lparen) {
					pass.Report(call, message)
					return true
				}
				start := pass.FileSet.Position(identifier.Pos()).Offset
				end := pass.FileSet.Position(identifier.End()).Offset
				pass.ReportFix(
					call,
					message,
					diagnostic.Fix{
						Message:   "replace append with its slice argument",
						Safety:    diagnostic.Safe,
						Automatic: true,
						Edits: []diagnostic.TextEdit{
							{
								Start:   start,
								End:     end,
								OldText: identifier.Name,
							},
						},
					},
				)
				return true
			},
		)
	}
}

func hasCommentBetween(groups []*ast.CommentGroup, start, end token.Pos) bool {
	for _, group := range groups {
		if group.Pos() >= start && group.End() <= end {
			return true
		}
	}
	return false
}
