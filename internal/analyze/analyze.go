package analyze

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/source"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const loadMode = packages.NeedName |
	packages.NeedFiles |
	packages.NeedCompiledGoFiles |
	packages.NeedImports |
	packages.NeedDeps |
	packages.NeedTypes |
	packages.NeedTypesSizes |
	packages.NeedSyntax |
	packages.NeedTypesInfo |
	packages.NeedModule

type target struct {
	path      string
	recursive bool
}

// Run loads the requested packages, executes the selected rules, and returns
// deterministic diagnostics. Directories are analyzed recursively, matching
// the path behavior of strider lint.
func Run(paths []string, registry *Registry) ([]diagnostic.Diagnostic, error) {
	patterns, targets, err := loadInputs(paths)
	if err != nil {
		return nil, err
	}
	loaded, err := packages.Load(
		&packages.Config{Mode: loadMode, Tests: true},
		patterns...,
	)
	if err != nil {
		return nil, err
	}
	if err := packageError(loaded); err != nil {
		return nil, err
	}
	var deprecatedObjects map[types.Object]string
	var deprecatedPackages map[*types.Package]string
	if registry.hasRule("deprecated-api-usage") {
		deprecatedObjects, deprecatedPackages = collectDeprecations(loaded)
	}
	builderMode := ssa.InstantiateGenerics
	if registry.hasRule("overwritten-before-use") || registry.hasRule("unchanged-loop-condition") ||
		registry.hasRule("never-nil-comparison") {
		builderMode |= ssa.GlobalDebug
	}
	ssaProgram, ssaPackages := ssautil.Packages(loaded, builderMode)
	ssaProgram.Build()
	functionsByPackage := collectPackageFunctions(ssaProgram, ssaPackages)
	seenFunctions := make(map[*ssa.Function]bool)
	for _, functions := range functionsByPackage {
		for _, function := range functions {
			seenFunctions[function] = true
		}
	}
	for function := range ssautil.AllFunctions(ssaProgram) {
		if function.Pkg != nil && !seenFunctions[function] {
			seenFunctions[function] = true
			functionsByPackage[function.Pkg] = append(
				functionsByPackage[function.Pkg],
				function,
			)
		}
	}

	diagnostics := make([]diagnostic.Diagnostic, 0)
	seenPackages := make(map[string]bool)
	seenDiagnostics := make(map[string]bool)
	generated := make(map[string]bool)
	generatedKnown := make(map[string]bool)
	for packageIndex, pkg := range loaded {
		if seenPackages[pkg.ID] {
			continue
		}
		seenPackages[pkg.ID] = true
		ssaPackage := ssaPackages[packageIndex]
		if ssaPackage == nil {
			continue
		}
		for _, rule := range registry.rules {
			meta := rule.Meta()
			goVersion := ""
			if pkg.Module != nil {
				goVersion = pkg.Module.GoVersion
			}
			pass := &Pass{
				PackagePath: pkg.PkgPath,
				GoVersion:   goVersion,
				Files:       pkg.Syntax,
				FileSet:     pkg.Fset,
				Types:       pkg.Types,
				TypesSizes:  pkg.TypesSizes,
				TypesInfo:   pkg.TypesInfo,
				SSAProgram:  ssaProgram,
				SSAPackage:  ssaPackage,
				Functions:   functionsByPackage[ssaPackage],

				deprecatedObjects:  deprecatedObjects,
				deprecatedPackages: deprecatedPackages,
			}
			pass.report = func(node ast.Node, message string) {
				position := pkg.Fset.Position(node.Pos())
				filename, pathErr := canonicalPath(position.Filename)
				if pathErr != nil || !matchesTarget(filename, targets) {
					return
				}
				if !generatedKnown[filename] {
					generated[filename], _ = source.IsGenerated(filename)
					generatedKnown[filename] = true
				}
				if generated[filename] {
					return
				}
				end := pkg.Fset.Position(node.End())
				display := source.DisplayPath(filename)
				position.Filename = display
				end.Filename = display
				key := fmt.Sprintf(
					"%s:%d:%d:%s:%s",
					filename,
					position.Offset,
					end.Offset,
					meta.Code,
					message,
				)
				if seenDiagnostics[key] {
					return
				}
				seenDiagnostics[key] = true
				diagnostics = append(
					diagnostics,
					diagnostic.Diagnostic{
						Code:     meta.Code,
						Message:  message,
						Severity: meta.DefaultSeverity,
						File:     display,
						Start:    position,
						End:      end,
					},
				)
			}
			rule.Run(pass)
		}
	}
	sortDiagnostics(diagnostics)
	return diagnostics, nil
}

func collectPackageFunctions(
	program *ssa.Program,
	ssaPackages []*ssa.Package,
) map[*ssa.Package][]*ssa.Function {
	functions := make(map[*ssa.Package][]*ssa.Function)
	seen := make(map[*ssa.Function]bool)
	var add func(*ssa.Function)
	add = func(function *ssa.Function) {
		if function == nil || function.Pkg == nil || seen[function] {
			return
		}
		seen[function] = true
		functions[function.Pkg] = append(functions[function.Pkg], function)
		for _, nested := range function.AnonFuncs {
			add(nested)
		}
	}
	for _, pkg := range ssaPackages {
		if pkg == nil {
			continue
		}
		for _, member := range pkg.Members {
			if function, ok := member.(*ssa.Function); ok {
				add(function)
			}
		}
		for _, name := range pkg.Pkg.Scope().Names() {
			typeName, ok := pkg.Pkg.Scope().Lookup(name).(*types.TypeName)
			if !ok {
				continue
			}
			named, ok := types.Unalias(typeName.Type()).(*types.Named)
			if !ok {
				continue
			}
			for _, receiver := range []types.Type{named, types.NewPointer(named)} {
				methodSet := types.NewMethodSet(receiver)
				for index := range methodSet.Len() {
					add(program.MethodValue(methodSet.At(index)))
				}
			}
		}
	}
	return functions
}

func loadInputs(paths []string) ([]string, []target, error) {
	if len(paths) == 0 {
		paths = []string{"."}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	if cwd, err = canonicalPath(cwd); err != nil {
		return nil, nil, err
	}
	patterns := make([]string, 0, len(paths))
	targets := make([]target, 0, len(paths))
	for _, input := range paths {
		path := filepath.Clean(input)
		slashInput := filepath.ToSlash(input)
		if slashInput == "..." || strings.HasSuffix(slashInput, "/...") {
			root := strings.TrimSuffix(slashInput, "/...")
			if root == "" {
				root = "."
			}
			path = filepath.FromSlash(root)
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, nil, fmt.Errorf("discover %q: %w", input, err)
		}
		absolute, err := canonicalPath(path)
		if err != nil {
			return nil, nil, err
		}
		if info.IsDir() {
			patterns = append(patterns, recursivePackagePattern(cwd, absolute))
			targets = append(targets, target{path: absolute, recursive: true})
			continue
		}
		if filepath.Ext(absolute) != ".go" {
			return nil, nil, fmt.Errorf("%q is not a Go source file", input)
		}
		patterns = append(patterns, "file="+filepath.ToSlash(absolute))
		targets = append(targets, target{path: absolute})
	}
	return patterns, targets, nil
}

func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return resolved, nil
	}
	return absolute, nil
}

func recursivePackagePattern(cwd, directory string) string {
	relative, err := filepath.Rel(cwd, directory)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		if relative == "." {
			return "./..."
		}
		return "./" + filepath.ToSlash(relative) + "/..."
	}
	return filepath.ToSlash(directory) + "/..."
}

func matchesTarget(filename string, targets []target) bool {
	for _, item := range targets {
		if filename == item.path {
			return true
		}
		if item.recursive && strings.HasPrefix(filename, item.path+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func packageError(loaded []*packages.Package) error {
	errors := make([]string, 0)
	for _, pkg := range loaded {
		for _, item := range pkg.Errors {
			errors = append(errors, item.Error())
		}
	}
	if len(errors) == 0 {
		return nil
	}
	sort.Strings(errors)
	return fmt.Errorf("load packages: %s", errors[0])
}

func sortDiagnostics(diagnostics []diagnostic.Diagnostic) {
	sort.Slice(
		diagnostics,
		func(i, j int) bool {
			left, right := diagnostics[i], diagnostics[j]
			if left.File != right.File {
				return left.File < right.File
			}
			if left.Start.Offset != right.Start.Offset {
				return left.Start.Offset < right.Start.Offset
			}
			if left.Code != right.Code {
				return left.Code < right.Code
			}
			return left.Message < right.Message
		},
	)
}
