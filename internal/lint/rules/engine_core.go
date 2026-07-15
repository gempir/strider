package rules

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strconv"
)

type analyzer struct {
	filename       string
	fset           *token.FileSet
	file           *ast.File
	content        []byte
	enabled        map[string]bool
	reporter       func(Finding)
	ancestors      []ast.Node
	current        ast.Node
	imports        map[string]string
	importNames    map[string]bool
	rangeVariables map[string]bool
}

// Analyze runs all selected native rules over one file.
func Analyze(input Input) {
	if len(input.Rules) == 0 {
		return
	}
	enabled := make(map[string]bool, len(input.Rules))
	for _, rule := range input.Rules {
		enabled[rule.Meta().Code] = true
	}
	a := &analyzer{
		filename:       input.Filename,
		fset:           input.FileSet,
		file:           input.File,
		content:        input.Content,
		enabled:        enabled,
		reporter:       input.Report,
		imports:        make(map[string]string),
		importNames:    make(map[string]bool),
		rangeVariables: make(map[string]bool),
	}
	a.indexImports()
	a.checkFile()
	stack := []ast.Node{}
	ast.Inspect(
		input.File,
		func(node ast.Node) bool {
			if node == nil {
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				return true
			}
			a.ancestors = stack
			a.current = node
			a.checkNode(node)
			stack = append(stack, node)
			return true
		},
	)
}

func (a *analyzer) on(code string) bool {
	return a.enabled[code]
}

func (a *analyzer) report(code string, node ast.Node, message string) {
	if a.on(code) && node != nil && a.reporter != nil {
		a.reporter(
			Finding{
				Node:      node,
				Scope:     a.current,
				Ancestors: a.ancestors,
				Code:      code,
				Message:   message,
			},
		)
	}
}

func (a *analyzer) indexImports() {
	for _, spec := range a.file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		a.imports[name] = path
		if name != "_" && name != "." {
			a.importNames[name] = true
		}
	}
}

func (a *analyzer) checkFile() {
	a.checkFilenameAndPackage()
	a.checkLinesAndComments()
	a.checkImports()
	a.checkPublicStructCount()
	a.checkRepeatedLiterals()
	a.checkConfusingNames()
	a.checkMarshalReceivers()
}
