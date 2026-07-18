package semantic

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/source"
)

const loadMode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule

type target struct {
	path string
	recursive bool
}

type analysisTask struct {
	pass *Pass
	rule Rule
}

type ssaBuildResult struct {
	program *ssa.Program
	packages []*ssa.Package
	functionsByPackage map[*ssa.Package][]*ssa.Function
}

type ssaBuildFunc func([]*packages.Package, ssa.BuilderMode) ssaBuildResult

type analysisFinding struct {
	key string
	diagnostic diagnostic.Diagnostic
}

type analysisFileInfo struct {
	filename string
	display string
	eligible bool
}

type analysisFileInfoCacheEntry struct {
	once sync.Once
	info analysisFileInfo
}

// Run loads the requested packages, executes the selected rules, and returns
// deterministic diagnostics. Directories are analyzed recursively, matching
// the path behavior of strider syntax.
func Run(paths []string, registry *Registry) ([]diagnostic.Diagnostic, error) {
	return run(paths, registry, buildSSA)
}

func run(paths []string, registry *Registry, ssaBuilder ssaBuildFunc) ([]diagnostic.Diagnostic, error) {
	if registry == nil {
		return nil, fmt.Errorf("analysis registry is nil")
	}
	patterns, targets, err := loadInputs(paths)
	if err != nil {
		return nil, err
	}
	if len(registry.rules) == 0 {
		return[]diagnostic.Diagnostic{}, nil
	}
	plan := registry.executionPlan()
	loaded, err := packages.Load(&packages.Config{Mode: loadMode, Tests: true}, patterns...)
	if err != nil {
		return nil, err
	}
	if err := packageError(loaded); err != nil {
		return nil, err
	}
	initial := selectInitialPackages(loaded)
	var deprecatedObjects map[types.Object]string
	var deprecatedPackages map[*types.Package]string
	if plan.requirements.Facts.Has(FactDeprecations) {
		deprecatedObjects, deprecatedPackages = collectDeprecations(loaded)
	}
	ssaResult := prepareSSA(initial, plan, ssaBuilder)
	ssaProgram := ssaResult.program
	ssaPackages := ssaResult.packages
	functionsByPackage := ssaResult.functionsByPackage
	needsSSA := plan.needsSSA()

	seenPackages := make(map[string]bool)
	passes := make([]*Pass, 0, len(initial))
	for packageIndex, pkg := range initial {
		if seenPackages[pkg.ID] {
			continue
		}
		seenPackages[pkg.ID] = true
		ssaPackage := ssaPackages[packageIndex]
		if needsSSA && ssaPackage == nil {
			continue
		}
		goVersion := ""
		if pkg.Module != nil {
			goVersion = pkg.Module.GoVersion
		}
		passes = append(
			passes,
			&Pass{
				PackagePath: pkg.PkgPath,
				GoVersion: goVersion,
				Files: pkg.Syntax,
				FileSet: pkg.Fset,
				Types: pkg.Types,
				TypesSizes: pkg.TypesSizes,
				TypesInfo: pkg.TypesInfo,
				SSAProgram: ssaProgram,
				SSAPackage: ssaPackage,
				Functions: functionsByPackage[ssaPackage],
				facts: newPackageFacts(plan.requirements.Facts, plan.staticCallPackages),

				deprecatedObjects: deprecatedObjects,
				deprecatedPackages: deprecatedPackages,
			},
		)
	}

	fileInfoCache := sync.Map{}
	fileInfoFor := func(filename string) analysisFileInfo {
		cached, ok := fileInfoCache.Load(filename)
		if !ok {
			cached, _ = fileInfoCache.LoadOrStore(filename, &analysisFileInfoCacheEntry{})
		}
		entry := cached.(*analysisFileInfoCacheEntry)
		entry.once.Do(
			func() {
				canonical,
				pathErr := canonicalPath(filename)
				if pathErr == nil && matchesTarget(canonical, targets) {
					generated,
					generatedErr := source.IsGenerated(canonical)
					if generatedErr == nil && !generated {
						entry.info = analysisFileInfo{filename: canonical, display: source.DisplayPath(canonical), eligible: true}
					}
				}
			},
		)
		return entry.info
	}

	taskCount := len(passes) * len(registry.rules)
	jobs := make(chan analysisTask, taskCount)
	results := make(chan[]analysisFinding, taskCount)
	workers := min(runtime.GOMAXPROCS(0), max(1, taskCount))
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for task := range jobs {
				results <- runAnalysisTask(task, registry, fileInfoFor)
			}
		}()
	}
	for _, pass := range passes {
		for _, rule := range registry.rules {
			jobs <- analysisTask{pass: pass, rule: rule}
		}
	}
	close(jobs)

	diagnostics := make([]diagnostic.Diagnostic, 0)
	seenDiagnostics := make(map[string]bool)
	for range taskCount {
		for _, finding := range <- results {
			if seenDiagnostics[finding.key] {
				continue
			}
			seenDiagnostics[finding.key] = true
			diagnostics = append(diagnostics, finding.diagnostic)
		}
	}
	group.Wait()
	close(results)
	sortDiagnostics(diagnostics)
	return diagnostics, nil
}

func (plan executionPlan) ssaBuilderMode() ssa.BuilderMode {
	mode := ssa.InstantiateGenerics
	if plan.requirements.SSAFeatures & SSAFeatureGlobalDebug != 0 {
		mode |= ssa.GlobalDebug
	}
	return mode
}

func prepareSSA(initial []*packages.Package, plan executionPlan, build ssaBuildFunc) ssaBuildResult {
	if !plan.needsSSA() {
		return ssaBuildResult{packages: make([]*ssa.Package, len(initial)), functionsByPackage: make(map[*ssa.Package][]*ssa.Function)}
	}
	return build(initial, plan.ssaBuilderMode())
}

func buildSSA(initial []*packages.Package, mode ssa.BuilderMode) ssaBuildResult {
	program, packages := ssautil.Packages(initial, mode)
	program.Build()
	functionsByPackage := collectPackageFunctions(program, packages)
	seenFunctions := make(map[*ssa.Function]bool)
	for _, functions := range functionsByPackage {
		for _, function := range functions {
			seenFunctions[function] = true
		}
	}
	for function := range ssautil.AllFunctions(program) {
		if function.Pkg != nil && !seenFunctions[function] {
			seenFunctions[function] = true
			functionsByPackage[function.Pkg] = append(functionsByPackage[function.Pkg], function)
		}
	}
	return ssaBuildResult{program: program, packages: packages, functionsByPackage: functionsByPackage}
}

// selectInitialPackages avoids analyzing production syntax twice when
// packages.Load returns both the ordinary package and a test-augmented package
// with the same import path. The augmented variant is the one with more syntax
// files; external test packages have a distinct import path and remain
// separate. Diagnostics from synthetic test-main packages are filtered by the
// requested source targets later in the pipeline.
func selectInitialPackages(loaded []*packages.Package) []*packages.Package {
	bestByPath := make(map[string]*packages.Package)
	for _, pkg := range loaded {
		if current := bestByPath[pkg.PkgPath]; current == nil || len(pkg.Syntax) > len(current.Syntax) {
			bestByPath[pkg.PkgPath] = pkg
		}
	}
	result := make([]*packages.Package, 0, len(loaded))
	for _, pkg := range loaded {
		if bestByPath[pkg.PkgPath] != pkg {
			continue
		}
		result = append(result, pkg)
	}
	return result
}

func runAnalysisTask(task analysisTask, registry *Registry, fileInfoFor func(string) analysisFileInfo) []analysisFinding {
	meta := task.rule.Meta()
	severity := registry.Severity(meta.Code)
	pass := *task.pass
	findings := []analysisFinding{}
	pass.report = func(node ast.Node, message string) {
		position := pass.FileSet.Position(node.Pos())
		info := fileInfoFor(position.Filename)
		if !info.eligible || registry.Excluded(meta.Code, info.filename) {
			return
		}
		end := pass.FileSet.Position(node.End())
		position.Filename = info.display
		end.Filename = info.display
		findings = append(
			findings,
			analysisFinding{
				key: fmt.Sprintf("%s:%d:%d:%s:%s", info.filename, position.Offset, end.Offset, meta.Code, message),
				diagnostic: diagnostic.Diagnostic{Code: meta.Code, Message: message, Severity: severity, File: info.display, Start: position, End: end},
			},
		)
	}
	task.rule.Run(&pass)
	return findings
}

func collectPackageFunctions(program *ssa.Program, ssaPackages []*ssa.Package) map[*ssa.Package][]*ssa.Function {
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
			for _, receiver := range[]types.Type{named, types.NewPointer(named)} {
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
		patterns = append(patterns, "file=" + filepath.ToSlash(absolute))
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
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".." + string(filepath.Separator)) {
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
		if item.recursive && strings.HasPrefix(filename, item.path + string(filepath.Separator)) {
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
	sort.SliceStable(
		diagnostics,
		func(i, j int) bool {
			left,
			right := diagnostics[i],
			diagnostics[j]
			if left.File != right.File {
				return left.File < right.File
			}
			if left.Start.Offset != right.Start.Offset {
				return left.Start.Offset < right.Start.Offset
			}
			if left.Code != right.Code {
				return left.Code < right.Code
			}
			if left.Message != right.Message {
				return left.Message < right.Message
			}
			if left.End.Offset != right.End.Offset {
				return left.End.Offset < right.End.Offset
			}
			return left.Severity < right.Severity
		},
	)
}
