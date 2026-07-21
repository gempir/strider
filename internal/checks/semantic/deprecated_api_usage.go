package semantic

import (
	"fmt"
	"go/ast"
	"go/types"
	"go/version"
	"runtime"
	"strconv"
	"strings"

	"github.com/gempir/strider/internal/diagnostic"
)

type deprecatedAPIUsageCheck struct{}

func (deprecatedAPIUsageCheck) Meta() Meta {
	return Meta{
		Code:            "deprecated-api-usage",
		Summary:         "detect uses of deprecated packages and APIs",
		Explanation:     "Go documentation marks packages, functions, methods, fields, variables, constants, and types with Deprecated paragraphs. Uses from other packages should migrate to the documented replacement.",
		GoodExample:     "value, err := io.ReadAll(reader)",
		BadExample:      "value, err := ioutil.ReadAll(reader)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (deprecatedAPIUsageCheck) Run(pass *Pass) {
	selectors := make(map[*ast.Ident]bool)
	pass.Inspect(
		[]ast.Node{
			(*ast.SelectorExpr)(nil),
		},
		func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			selectors[selector.Sel] = true
			reportDeprecatedObject(pass, selector, selector.Sel)
			return true
		},
	)
	pass.Inspect(
		[]ast.Node{
			(*ast.Ident)(nil),
			(*ast.ImportSpec)(nil),
		},
		func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.ImportSpec:
				reportDeprecatedImport(pass, node)
			case *ast.Ident:
				if !selectors[node] {
					reportDeprecatedObject(pass, node, node)
				}
			}
			return true
		},
	)
}

func reportDeprecatedImport(pass *Pass, spec *ast.ImportSpec) {
	var imported *types.Package
	if spec.Name != nil {
		if name, ok := pass.TypesInfo.Defs[spec.Name].(*types.PkgName); ok {
			imported = name.Imported()
		}
	} else if name, ok := pass.TypesInfo.Implicits[spec].(*types.PkgName); ok {
		imported = name.Imported()
	}
	if imported == nil || relatedPackage(pass.PackagePath, imported.Path()) {
		return
	}
	message := pass.facts.deprecatedPackages[imported]
	if message == "" || suppressStandardLibraryDeprecation(pass, imported.Path()) {
		return
	}
	path, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		path = spec.Path.Value
	}
	pass.Report(spec.Path, fmt.Sprintf("%s is deprecated: %s", path, message))
}

func reportDeprecatedObject(pass *Pass, node ast.Node, identifier *ast.Ident) {
	object := pass.TypesInfo.Uses[identifier]
	if object == nil || object.Pkg() == nil || relatedPackage(pass.PackagePath, object.Pkg().Path()) {
		return
	}
	message := deprecatedObjectMessage(pass.facts.deprecatedObjects, object)
	if message == "" || suppressStandardLibraryDeprecation(pass, object.Pkg().Path()) {
		return
	}
	pass.Report(node, fmt.Sprintf("%s.%s is deprecated: %s", object.Pkg().Path(), object.Name(), message))
}

func deprecatedObjectMessage(messages map[types.Object]string, object types.Object) string {
	if message := messages[object]; message != "" {
		return message
	}
	switch object := object.(type) {
	case *types.Func:
		return messages[object.Origin()]
	case *types.Var:
		return messages[object.Origin()]
	}
	return ""
}

func relatedPackage(current, used string) bool {
	current = strings.TrimSuffix(strings.TrimSuffix(current, "_test"), ".test")
	used = strings.TrimSuffix(strings.TrimSuffix(used, "_test"), ".test")
	return current == used
}

func suppressStandardLibraryDeprecation(pass *Pass, packagePath string) bool {
	if strings.Contains(packagePath, ".") || pass.GoVersion == "" {
		return false
	}
	target := normalizeGoVersion(pass.GoVersion)
	running := languageGoVersion(runtime.Version())
	if index := strings.IndexAny(running, " -"); index >= 0 {
		running = running[:index]
	}
	return version.IsValid(target) && version.IsValid(running) && version.Compare(target, running) < 0
}

func languageGoVersion(value string) string {
	value = strings.TrimPrefix(value, "go")
	parts := strings.Split(value, ".")
	if len(parts) < 2 {
		return "go" + value
	}
	return "go" + parts[0] + "." + parts[1]
}

func (deprecatedAPIUsageCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
		Facts: FactDeprecations,
	}
}
