package analyze

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"go/version"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/gempir/strider/internal/diagnostic"
)

type deprecatedAPIUsageRule struct {}

func (deprecatedAPIUsageRule) Meta() Meta {
	return Meta{
		Code: "deprecated-api-usage",
		Summary: "detect uses of deprecated packages and APIs",
		Explanation: "Go documentation marks packages, functions, methods, fields, variables, constants, and types with Deprecated paragraphs. Uses from other packages should migrate to the documented replacement.",
		GoodExample: "value, err := io.ReadAll(reader)",
		BadExample: "value, err := ioutil.ReadAll(reader)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (deprecatedAPIUsageRule) Run(pass *Pass) {
	selectors := make(map[*ast.Ident]bool)
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				selector,
				ok := node.(*ast.SelectorExpr)
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

type deprecationSourceKey struct {
	line int
	name string
	owner string
}

type parsedDeprecationFile struct {
	declarations map[deprecationSourceKey]string
	packageMessage string
}

type deprecationIndex struct {
	objects map[types.Object]string
	packages map[*types.Package]string
	seenObjects map[types.Object]bool
	declarationFiles map[string]parsedDeprecationFile
	packageClauseFiles map[string]string
	packageClauseFilesSeen map[string]bool
	packageDirectories map[string]string
	packageDirectoriesSeen map[string]bool
	physicalFiles map[*types.Package][]string
}

func collectDeprecations(roots []*packages.Package) (
	map[types.Object]string,
	map[*types.Package]string,
) {
	index := deprecationIndex{
		objects: make(map[types.Object]string),
		packages: make(map[*types.Package]string),
		seenObjects: make(map[types.Object]bool),
		declarationFiles: make(map[string]parsedDeprecationFile),
		packageClauseFiles: make(map[string]string),
		packageClauseFilesSeen: make(map[string]bool),
		packageDirectories: make(map[string]string),
		packageDirectoriesSeen: make(map[string]bool),
		physicalFiles: make(map[*types.Package][]string),
	}
	packages.Visit(
		roots,
		nil,
		func(pkg *packages.Package) {
			if pkg == nil || pkg.Types == nil {
				return
			}
			files := pkg.CompiledGoFiles
			if len(files) == 0 {
				files = pkg.GoFiles
			}
			for _,
			filename := range files {
				index.physicalFiles[pkg.Types] = append(
					index.physicalFiles[pkg.Types],
					expandGoRoot(filename),
				)
			}
		},
	)
	for _, root := range roots {
		if root == nil || root.Fset == nil || root.TypesInfo == nil {
			continue
		}
		for _, object := range root.TypesInfo.Uses {
			if object == nil || object.Pkg() == nil || object.Pkg() == root.Types {
				continue
			}
			index.addObject(root.Fset, object)
		}
	}
	for _, root := range roots {
		if root == nil || root.Types == nil {
			continue
		}
		for _, imported := range root.Imports {
			if imported == nil || imported.Types == nil {
				continue
			}
			index.addPackage(imported)
		}
	}
	return index.objects, index.packages
}

func (index *deprecationIndex) addObject(fileSet *token.FileSet, object types.Object) {
	declaration := declarationObject(object)
	if index.seenObjects[declaration] {
		return
	}
	index.seenObjects[declaration] = true
	position := fileSet.Position(declaration.Pos())
	if position.Filename == "" || position.Line == 0 {
		return
	}
	filename := expandGoRoot(position.Filename)
	parsed := index.declarationFile(filename)
	needsPhysicalFallback := !index.isPhysicalFile(declaration.Pkg(), filename)
	for _, owner := range deprecationObjectOwners(declaration) {
		key := deprecationSourceKey{line: position.Line, name: declaration.Name(), owner: owner}
		message := parsed.declarations[key]
		if message == "" && needsPhysicalFallback {
			message = index.physicalDeclarationMessage(declaration.Pkg(), key)
		}
		if message != "" {
			index.objects[declaration] = message
			return
		}
	}
}

func (index *deprecationIndex) isPhysicalFile(pkg *types.Package, filename string) bool {
	if pkg == nil {
		return false
	}
	filename = filepath.Clean(filename)
	for _, candidate := range index.physicalFiles[pkg] {
		if filepath.Clean(candidate) == filename {
			return true
		}
	}
	return false
}

func (index *deprecationIndex) physicalDeclarationMessage(
	pkg *types.Package,
	key deprecationSourceKey,
) string {
	if pkg == nil {
		return ""
	}
	for _, filename := range index.physicalFiles[pkg] {
		parsed := index.declarationFile(filename)
		if message := parsed.declarations[key]; message != "" {
			return message
		}
	}
	return ""
}

func declarationObject(object types.Object) types.Object {
	switch object := object.(type) {
	case *types.Func:
		return object.Origin()
	case *types.Var:
		return object.Origin()
	default:
		return object
	}
}

func deprecationObjectOwners(object types.Object) []string {
	switch object := object.(type) {
	case *types.Func:
		signature, _ := object.Type().(*types.Signature)
		if signature != nil && signature.Recv() != nil {
			if name := deprecationReceiverTypeName(signature.Recv().Type()); name != "" {
				return[]string{name}
			}
			return nil
		}
		if object.Parent() == nil {
			return objectTypeOwners(object)
		}
	case *types.Var:
		if object.IsField() {
			return objectTypeOwners(object)
		}
	}
	return[]string{""}
}

func deprecationReceiverTypeName(value types.Type) string {
	if pointer, ok := value.(*types.Pointer); ok {
		value = pointer.Elem()
	}
	if alias, ok := value.(*types.Alias); ok {
		return alias.Obj().Name()
	}
	if named, ok := value.(*types.Named); ok {
		return named.Obj().Name()
	}
	return ""
}

func objectTypeOwners(object types.Object) []string {
	pkg := object.Pkg()
	if pkg == nil {
		return nil
	}
	owners := make([]string, 0, 1)
	for _, name := range pkg.Scope().Names() {
		typeName, ok := pkg.Scope().Lookup(name).(*types.TypeName)
		if ok && typeOwnsObject(typeName.Type(), object) {
			owners = append(owners, name)
		}
	}
	return owners
}

func typeOwnsObject(value types.Type, object types.Object) bool {
	value = types.Unalias(value)
	if named, ok := value.(*types.Named); ok {
		value = named.Underlying()
	}
	switch value := value.(type) {
	case *types.Struct:
		for field := range value.Fields() {
			if declarationObject(field) == object {
				return true
			}
		}
	case *types.Interface:
		value.Complete()
		for method := range value.ExplicitMethods() {
			if declarationObject(method) == object {
				return true
			}
		}
	}
	return false
}

func (index *deprecationIndex) declarationFile(filename string) parsedDeprecationFile {
	if parsed, ok := index.declarationFiles[filename]; ok {
		return parsed
	}
	parsed := parsedDeprecationFile{declarations: make(map[deprecationSourceKey]string)}
	fileSet := token.NewFileSet()
	file, _ := parser.ParseFile(
		fileSet,
		filename,
		nil,
		parser.ParseComments | parser.SkipObjectResolution,
	)
	if file != nil {
		parsed.packageMessage = deprecationMessage(file.Doc)
		collectFileDeprecations(fileSet, file, parsed.declarations)
	}
	index.declarationFiles[filename] = parsed
	return parsed
}

func collectFileDeprecations(
	fileSet *token.FileSet,
	file *ast.File,
	declarations map[deprecationSourceKey]string,
) {
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			addDeprecatedDeclaration(
				fileSet,
				declarations,
				declaration.Name,
				astReceiverTypeName(declaration.Recv),
				deprecationMessage(declaration.Doc),
			)
		case *ast.GenDecl:
			for _, rawSpec := range declaration.Specs {
				switch spec := rawSpec.(type) {
				case *ast.ValueSpec:
					message := firstDeprecation(spec.Doc, spec.Comment, declaration.Doc)
					for _, name := range spec.Names {
						addDeprecatedDeclaration(fileSet, declarations, name, "", message)
					}
				case *ast.TypeSpec:
					message := firstDeprecation(spec.Doc, spec.Comment, declaration.Doc)
					addDeprecatedDeclaration(fileSet, declarations, spec.Name, "", message)
					collectFieldDeprecations(fileSet, spec.Type, spec.Name.Name, declarations)
				}
			}
		}
	}
}

func collectFieldDeprecations(
	fileSet *token.FileSet,
	expression ast.Expr,
	owner string,
	declarations map[deprecationSourceKey]string,
) {
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
			addDeprecatedDeclaration(fileSet, declarations, name, owner, message)
		}
	}
}

func addDeprecatedDeclaration(
	fileSet *token.FileSet,
	declarations map[deprecationSourceKey]string,
	name *ast.Ident,
	owner string,
	message string,
) {
	if name != nil && message != "" {
		declarations[deprecationSourceKey{
			line: fileSet.Position(name.Pos()).Line,
			name: name.Name,
			owner: owner,
		}] = message
	}
}

func astReceiverTypeName(fields *ast.FieldList) string {
	if fields == nil || len(fields.List) != 1 {
		return ""
	}
	expression := fields.List[0].Type
	for {
		switch current := expression.(type) {
		case *ast.Ident:
			return current.Name
		case *ast.ParenExpr:
			expression = current.X
		case *ast.StarExpr:
			expression = current.X
		case *ast.IndexExpr:
			expression = current.X
		case *ast.IndexListExpr:
			expression = current.X
		default:
			return ""
		}
	}
}

func (index *deprecationIndex) addPackage(pkg *packages.Package) {
	if _, seen := index.packages[pkg.Types]; seen {
		return
	}
	files := pkg.CompiledGoFiles
	if len(files) == 0 {
		files = pkg.GoFiles
	}
	if len(files) == 0 {
		return
	}
	directory := pkg.Dir
	if directory == "" {
		directory = filepath.Dir(files[0])
	}
	key := directory + "\x00" + strings.Join(files, "\x00")
	if !index.packageDirectoriesSeen[key] {
		message := ""
		for _, filename := range files {
			if candidate := index.packageClauseMessage(expandGoRoot(filename)); candidate != "" {
				message = candidate
			}
		}
		index.packageDirectories[key] = message
		index.packageDirectoriesSeen[key] = true
	}
	if message := index.packageDirectories[key]; message != "" {
		index.packages[pkg.Types] = message
	}
}

func (index *deprecationIndex) packageClauseMessage(filename string) string {
	if parsed, ok := index.declarationFiles[filename]; ok {
		return parsed.packageMessage
	}
	if index.packageClauseFilesSeen[filename] {
		return index.packageClauseFiles[filename]
	}
	fileSet := token.NewFileSet()
	file, _ := parser.ParseFile(
		fileSet,
		filename,
		nil,
		parser.PackageClauseOnly | parser.ParseComments | parser.SkipObjectResolution,
	)
	message := ""
	if file != nil {
		message = deprecationMessage(file.Doc)
	}
	index.packageClauseFiles[filename] = message
	index.packageClauseFilesSeen[filename] = true
	return message
}

func expandGoRoot(filename string) string {
	const marker = "$GOROOT"
	if filename == marker {
		return runtime.GOROOT()
	}
	if strings.HasPrefix(filename, marker + "/") || strings.HasPrefix(filename, marker + "\\") {
		return filepath.Join(
			runtime.GOROOT(),
			filepath.FromSlash(strings.TrimLeft(filename[len(marker):], "/\\")),
		)
	}
	return filename
}

func firstDeprecation(groups... *ast.CommentGroup) string {
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
