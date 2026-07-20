package semantic

import (
	"go/ast"
	"go/token"

	"github.com/gempir/strider/internal/diagnostic"
)

type topLevelDeclarationOrderCheck struct{}

func (topLevelDeclarationOrderCheck) Meta() Meta {
	return Meta{
		Code:            "top-level-declaration-order",
		Summary:         "keep top-level declarations in const, var, type, and func order",
		Explanation:     "A consistent top-level declaration order makes files easier to scan. Group constants first, then variables, types, and functions; imports are ignored and init remains in the function group.",
		GoodExample:     "const timeout = 1; var defaultClient Client; type Client struct{}; func New() Client { return Client{} }",
		BadExample:      "var defaultClient Client; type Client struct{}",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (topLevelDeclarationOrderCheck) Run(pass *Pass) {
	for _, file := range pass.Files {
		highest := -1
		for _, declaration := range file.Decls {
			rank := declarationKindRank(declaration)
			if rank < 0 {
				continue
			}
			if rank < highest {
				pass.Report(declaration, "top-level declarations should be ordered as const, var, type, then func")
				break
			}
			if rank > highest {
				highest = rank
			}
		}
	}
}

func declarationKindRank(declaration ast.Decl) int {
	switch declaration := declaration.(type) {
	case *ast.GenDecl:
		switch declaration.Tok {
		case token.IMPORT:
			return -1
		case token.CONST:
			return 0
		case token.VAR:
			return 1
		case token.TYPE:
			return 2
		default:
			return -1
		}
	case *ast.FuncDecl:
		return 3
	default:
		return -1
	}
}

func (topLevelDeclarationOrderCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
