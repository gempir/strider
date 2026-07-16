package analyze

import (
	"fmt"
	"go/ast"
	"go/types"
	"go/version"
	"runtime"
	"strconv"
	"strings"

	"github.com/gempir/strider/internal/diagnostic"
	"golang.org/x/tools/go/packages"
)

type deprecatedAPIUsageRule struct{}

func (deprecatedAPIUsageRule) Meta() Meta {
	return Meta{
		Code:            "deprecated-api-usage",
		Summary:         "detect uses of deprecated packages and APIs",
		Explanation:     "Go documentation marks packages, functions, methods, fields, variables, constants, and types with Deprecated paragraphs. Uses from other packages should migrate to the documented replacement.",
		GoodExample:     "value, err := io.ReadAll(reader)",
		BadExample:      "value, err := ioutil.ReadAll(reader)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (deprecatedAPIUsageRule) Run(pass *Pass) {
	selectors := make(map[*ast.Ident]bool)
	for _, file := range pass.Files {
		ast.Inspect(
			file,
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
	}
	for _, file := range pass.Files {
		ast.Inspect(
			file,
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
	message := pass.deprecatedPackages[imported]
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
	message := deprecatedObjectMessage(pass.deprecatedObjects, object)
	if message == "" || suppressStandardLibraryDeprecation(pass, object.Pkg().Path()) {
		return
	}
	pass.Report(
		node,
		fmt.Sprintf("%s.%s is deprecated: %s", object.Pkg().Path(), object.Name(), message),
	)
}

func deprecatedObjectMessage(messages map[types.Object]string, object types.Object) string {
	if message := messages[object]; message != "" {
		return message
	}
	if function, ok := object.(*types.Func); ok {
		return messages[function.Origin()]
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
	return version.IsValid(target) && version.IsValid(running) &&
		version.Compare(target, running) < 0
}

func languageGoVersion(value string) string {
	value = strings.TrimPrefix(value, "go")
	parts := strings.Split(value, ".")
	if len(parts) < 2 {
		return "go" + value
	}
	return "go" + parts[0] + "." + parts[1]
}

func collectDeprecations(roots []*packages.Package) (map[types.Object]string, map[*types.Package]string) {
	objects := make(map[types.Object]string)
	packagesByType := make(map[*types.Package]string)
	seen := make(map[string]bool)
	var visit func(*packages.Package)
	visit = func(pkg *packages.Package) {
		if pkg == nil || seen[pkg.ID] {
			return
		}
		seen[pkg.ID] = true
		for _, imported := range pkg.Imports {
			visit(imported)
		}
		if pkg.TypesInfo == nil || pkg.Types == nil {
			return
		}
		for _, file := range pkg.Syntax {
			if message := deprecationMessage(file.Doc); message != "" {
				packagesByType[pkg.Types] = message
			}
			collectFileDeprecations(pkg.TypesInfo, file, objects)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	return objects, packagesByType
}

func collectFileDeprecations(info *types.Info, file *ast.File, objects map[types.Object]string) {
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			addDeprecatedObject(objects, info.Defs[declaration.Name], deprecationMessage(declaration.Doc))
		case *ast.GenDecl:
			for _, rawSpec := range declaration.Specs {
				switch spec := rawSpec.(type) {
				case *ast.ValueSpec:
					message := firstDeprecation(spec.Doc, spec.Comment, declaration.Doc)
					for _, name := range spec.Names {
						addDeprecatedObject(objects, info.Defs[name], message)
					}
				case *ast.TypeSpec:
					message := firstDeprecation(spec.Doc, spec.Comment, declaration.Doc)
					addDeprecatedObject(objects, info.Defs[spec.Name], message)
					collectFieldDeprecations(info, spec.Type, objects)
				}
			}
		}
	}
}

func collectFieldDeprecations(info *types.Info, expression ast.Expr, objects map[types.Object]string) {
	var fields *ast.FieldList
	switch expression := expression.(type) {
	case *ast.StructType:
		fields = expression.Fields
	case *ast.InterfaceType:
		fields = expression.Methods
	}
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		message := firstDeprecation(field.Doc, field.Comment)
		for _, name := range field.Names {
			addDeprecatedObject(objects, info.Defs[name], message)
		}
	}
}

func addDeprecatedObject(objects map[types.Object]string, object types.Object, message string) {
	if object != nil && message != "" {
		objects[object] = message
	}
}

func firstDeprecation(groups ...*ast.CommentGroup) string {
	for _, group := range groups {
		if message := deprecationMessage(group); message != "" {
			return message
		}
	}
	return ""
}

func deprecationMessage(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	text := group.Text()
	for _, paragraph := range strings.Split(text, "\n\n") {
		paragraph = strings.TrimSpace(paragraph)
		if strings.HasPrefix(paragraph, "Deprecated:") {
			message := strings.TrimSpace(strings.TrimPrefix(paragraph, "Deprecated:"))
			return strings.Join(strings.Fields(message), " ")
		}
	}
	return ""
}
