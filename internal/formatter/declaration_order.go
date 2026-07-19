package formatter

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

type topLevelDeclaration struct {
	rank  int
	start int
	end   int
	text  []byte
}

// orderTopLevelDeclarations applies the selected const, var, type, func file
// order. Declarations retain their relative order within each kind so package
// variable initialization and init function order remain unchanged.
func orderTopLevelDeclarations(filename string, source []byte) ([]byte, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filename, source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	declarations := make([]topLevelDeclaration, 0, len(file.Decls))
	ordered := true
	previousRank := -1
	for _, declaration := range file.Decls {
		rank := formatterDeclarationRank(declaration)
		if rank < 0 {
			continue
		}
		start := fileSet.Position(declaration.Pos()).Offset
		if documentation := declarationDocumentation(declaration); documentation != nil {
			start = fileSet.Position(documentation.Pos()).Offset
		}
		endPosition := fileSet.Position(declaration.End())
		end := endPosition.Offset
		for _, group := range file.Comments {
			groupStart := fileSet.Position(group.Pos())
			if groupStart.Offset < end || groupStart.Line != endPosition.Line {
				continue
			}
			end = fileSet.Position(group.End()).Offset
			break
		}
		declarations = append(declarations, topLevelDeclaration{
			rank:  rank,
			start: start,
			end:   end,
		})
		if rank < previousRank {
			ordered = false
		}
		previousRank = rank
	}
	if ordered || len(declarations) < 2 {
		return source, nil
	}
	for index := 1; index < len(declarations); index++ {
		gapStart := declarations[index-1].end
		gap := source[gapStart:declarations[index].start]
		if content := bytes.TrimLeft(gap, " \t\r\n"); len(content) != 0 {
			// A detached comment travels with the following declaration while its
			// internal blank-line separation remains intact.
			declarations[index].start = gapStart + len(gap) - len(content)
		}
	}

	firstStart := declarations[0].start
	suffix := bytes.TrimSpace(source[declarations[len(declarations)-1].end:])
	for index := range declarations {
		declaration := &declarations[index]
		declaration.text = bytes.TrimSpace(source[declaration.start:declaration.end])
	}
	sort.SliceStable(declarations, func(left, right int) bool {
		return declarations[left].rank < declarations[right].rank
	})

	var output strings.Builder
	prefix := bytes.TrimRight(source[:firstStart], " \t\r\n")
	output.Grow(len(source))
	output.Write(prefix)
	if len(prefix) != 0 {
		output.WriteString("\n\n")
	}
	for index, declaration := range declarations {
		if index != 0 {
			output.WriteString("\n\n")
		}
		output.Write(declaration.text)
	}
	if len(suffix) != 0 {
		output.WriteString("\n\n")
		output.Write(suffix)
	}
	output.WriteByte('\n')
	return []byte(output.String()), nil
}

func formatterDeclarationRank(declaration ast.Decl) int {
	switch current := declaration.(type) {
	case *ast.GenDecl:
		switch current.Tok {
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

func declarationDocumentation(declaration ast.Decl) *ast.CommentGroup {
	switch current := declaration.(type) {
	case *ast.GenDecl:
		return current.Doc
	case *ast.FuncDecl:
		return current.Doc
	default:
		return nil
	}
}
