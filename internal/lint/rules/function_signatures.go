package rules

import (
	"go/ast"
	"strings"
)

func (a *analyzer) checkExportedFunction(fn *ast.FuncDecl) {
	if !ast.IsExported(fn.Name.Name) || a.file.Name.Name == "main" ||
		(strings.HasSuffix(a.filename, "_test.go") &&
			(strings.HasPrefix(fn.Name.Name, "Test") || strings.HasPrefix(fn.Name.Name, "Benchmark") ||
				strings.HasPrefix(fn.Name.Name, "Example"))) {
		return
	}
	if fn.Recv != nil &&
		(fn.Name.Name == "Error" || fn.Name.Name == "Read" || fn.Name.Name == "ServeHTTP" ||
			fn.Name.Name == "String" ||
			fn.Name.Name == "Write" ||
			fn.Name.Name == "Unwrap") {
		return
	}
	if fn.Doc == nil || !strings.HasPrefix(strings.TrimSpace(fn.Doc.Text()), fn.Name.Name) {
		a.report(
			"exported",
			fn.Name,
			"exported function or method should have a comment beginning with its name",
		)
	}
}

func (a *analyzer) checkFunctionResults(fn *ast.FuncDecl) {
	results := fn.Type.Results
	if results == nil {
		return
	}
	previous := ""
	for index, field := range results.List {
		typeName := exprText(field.Type)
		if typeName == "error" && index != len(results.List)-1 {
			a.report("error-return", field.Type, "error should be the last returned value")
		}
		if ast.IsExported(fn.Name.Name) {
			base := strings.TrimLeft(typeName, "*[]")
			if identifierName(base) && base != "error" && !ast.IsExported(base) &&
				!builtinType(base) {
				a.report(
					"unexported-return",
					field.Type,
					"exported function returns an unexported type",
				)
			}
		}
		if len(field.Names) == 0 && previous == typeName {
			a.report(
				"confusing-results",
				field.Type,
				"adjacent unnamed results of the same type should be named",
			)
		}
		previous = typeName
	}
}

func (a *analyzer) checkContextArgument(fn *ast.FuncDecl) {
	if fn.Type.Params == nil {
		return
	}
	position := 0
	for _, field := range fn.Type.Params.List {
		if exprText(field.Type) == "context.Context" && position != 0 {
			a.report(
				"context-as-argument",
				field.Type,
				"context.Context should be the first parameter",
			)
		}
		position += max(1, len(field.Names))
	}
}

func (a *analyzer) checkFunctionNames(fn *ast.FuncDecl) {
	checkFieldNames := func(list *ast.FieldList) {
		if list == nil {
			return
		}
		for _, field := range list.List {
			if exprText(field.Type) == "sync.WaitGroup" {
				a.report(
					"waitgroup-by-value",
					field.Type,
					"sync.WaitGroup should be passed by pointer",
				)
			}
			if exprText(field.Type) == "time.Duration" {
				for _, name := range field.Names {
					if hasTimeUnitSuffix(name.Name) {
						a.report(
							"time-naming",
							name,
							"time.Duration name should not include a unit suffix",
						)
					}
				}
			}
			for _, name := range field.Names {
				a.checkIdentifierName(name)
			}
		}
	}
	checkFieldNames(fn.Type.Params)
	checkFieldNames(fn.Type.Results)
	if fn.Recv != nil {
		for _, field := range fn.Recv.List {
			for _, name := range field.Names {
				a.checkIdentifierName(name)
			}
		}
	}
}
