package analyze

import (
	"fmt"
	"go/ast"
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
	ssaProgram, ssaPackages := ssautil.Packages(loaded, ssa.InstantiateGenerics)
	ssaProgram.Build()
	functionsByPackage := make(map[*ssa.Package][]*ssa.Function)
	for function := range ssautil.AllFunctions(ssaProgram) {
		if function.Pkg != nil {
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
			pass := &Pass{
				PackagePath: pkg.PkgPath,
				Files:       pkg.Syntax,
				FileSet:     pkg.Fset,
				Types:       pkg.Types,
				TypesInfo:   pkg.TypesInfo,
				SSAProgram:  ssaProgram,
				SSAPackage:  ssaPackage,
				Functions:   functionsByPackage[ssaPackage],
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
