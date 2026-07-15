package rules

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
)

func (a *analyzer) checkRepeatedLiterals() {
	counts := map[string][]*ast.BasicLit{}
	ast.Inspect(
		a.file,
		func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.GenDecl:
				return false
			case *ast.BasicLit:
				if n.Kind == token.STRING {
					value, _ := strconv.Unquote(n.Value)
					if value != "" {
						counts[n.Value] = append(counts[n.Value], n)
					}
				}
			}
			return true
		},
	)
	for literal, nodes := range counts {
		if len(nodes) > 2 {
			a.report(
				"add-constant",
				nodes[2],
				fmt.Sprintf("string literal %s appears more than twice; define a constant", literal),
			)
		}
	}
}
