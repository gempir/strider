package semantic

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const probeFingerprintVersion = "strider-analyze-probe-v3"

// analysisSessionFingerprinter keeps only file paths and package metadata. It
// never retains syntax, types, or SSA. The expensive go/packages metadata
// query is repeated after a detected change, while unchanged iterations prove
// identity by hashing the prior graph's complete inputs directly.
type analysisSessionFingerprinter struct {
	probe *analysisProbe
}

type analysisProbeScope struct {
	key            analysisCacheKey
	cwd            string
	patterns       []string
	recursiveRoots []string
}

type analysisProbe struct {
	scope                analysisProbeScope
	static               analysisCacheKey
	resolvedEnvironment  []byte
	requiredFiles        []string
	optionalFiles        []string
	directories          []string
	recursiveDirectories []string
	packageDirectories   map[string]bool
	baseline             analysisCacheKey
}

func (fingerprinter *analysisSessionFingerprinter) fingerprint(paths []string, registry *Registry) (analysisCacheKey, error) {
	scope, err := newAnalysisProbeScope(paths, registry)
	if err != nil {
		return analysisCacheKey{}, err
	}
	if fingerprinter.probe != nil && fingerprinter.probe.scope.key == scope.key {
		current, currentErr := fingerprinter.probe.currentKey()
		if currentErr == nil && current == fingerprinter.probe.baseline {
			return current, nil
		}
	}
	probe, err := buildAnalysisProbe(scope)
	if err != nil {
		return analysisCacheKey{}, err
	}
	fingerprinter.probe = probe
	return probe.baseline, nil
}

func (fingerprinter *analysisSessionFingerprinter) reset() {
	fingerprinter.probe = nil
}

func newAnalysisProbeScope(paths []string, registry *Registry) (analysisProbeScope, error) {
	if registry == nil {
		return analysisProbeScope{}, fmt.Errorf("fingerprint analysis: nil registry")
	}
	patterns, targets, err := loadInputs(paths)
	if err != nil {
		return analysisProbeScope{}, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return analysisProbeScope{}, err
	}
	if cwd, err = canonicalPath(cwd); err != nil {
		return analysisProbeScope{}, err
	}
	effectivePaths := paths
	if len(effectivePaths) == 0 {
		effectivePaths = []string{"."}
	}
	recursiveRoots := make(map[string]bool)
	for _, item := range targets {
		if item.recursive {
			recursiveRoots[item.path] = true
		}
	}
	roots := mapKeys(recursiveRoots)
	sort.Strings(roots)
	writer := newFingerprintWriter()
	writer.addString(probeFingerprintVersion)
	writer.addString(cwd)
	writer.addStrings(effectivePaths)
	writer.addStrings(patterns)
	writer.addStrings(roots)
	addRegistryFingerprint(writer, registry)
	return analysisProbeScope{key: writer.sum(), cwd: cwd, patterns: patterns, recursiveRoots: roots}, nil
}

func buildAnalysisProbe(scope analysisProbeScope) (*analysisProbe, error) {
	_, resolved, err := analysisEnvironment(scope.cwd)
	if err != nil {
		return nil, err
	}
	loaded, err := packages.Load(&packages.Config{Mode: fingerprintLoadMode, Tests: true}, scope.patterns...)
	if err != nil {
		return nil, err
	}
	if err := packageError(loaded); err != nil {
		return nil, err
	}
	probe := &analysisProbe{scope: scope, resolvedEnvironment: resolved}
	staticWriter := newFingerprintWriter()
	staticWriter.addString(probeFingerprintVersion)
	staticWriter.addUint64(uint64(len(loaded)))
	for _, root := range loaded {
		staticWriter.addString(root.ID)
	}
	probe.collectPackageGraph(staticWriter, loaded)
	probe.static = staticWriter.sum()
	if err := probe.collectConfigurationFiles(); err != nil {
		return nil, err
	}
	probe.normalize()
	probe.baseline, err = probe.currentKey()
	if err != nil {
		return nil, err
	}
	return probe, nil
}

func (probe *analysisProbe) collectPackageGraph(writer *fingerprintWriter, roots []*packages.Package) {
	seen := make(map[*packages.Package]bool)
	var visit func(*packages.Package)
	visit = func(pkg *packages.Package) {
		if pkg == nil || seen[pkg] {
			return
		}
		seen[pkg] = true
		for _, imported := range pkg.Imports {
			visit(imported)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	all := make([]*packages.Package, 0, len(seen))
	for pkg := range seen {
		all = append(all, pkg)
	}
	sort.Slice(
		all,
		func(leftIndex, rightIndex int) bool {
			left,
				right := all[leftIndex],
				all[rightIndex]
			if left.ID != right.ID {
				return left.ID < right.ID
			}
			if left.PkgPath != right.PkgPath {
				return left.PkgPath < right.PkgPath
			}
			return left.Dir < right.Dir
		},
	)
	files := make(map[string]bool)
	directories := make(map[string]bool)
	recursiveDirectories := make(map[string]bool)
	for _, pkg := range all {
		writer.addString(pkg.ID)
		writer.addString(pkg.Name)
		writer.addString(pkg.PkgPath)
		writer.addString(pkg.Dir)
		writer.addStrings(pkg.GoFiles)
		writer.addStrings(pkg.CompiledGoFiles)
		writer.addStrings(pkg.OtherFiles)
		writer.addStrings(pkg.EmbedFiles)
		writer.addStrings(pkg.EmbedPatterns)
		writer.addStrings(pkg.IgnoredFiles)
		addFiles(files, pkg.GoFiles)
		addFiles(files, pkg.CompiledGoFiles)
		addFiles(files, pkg.OtherFiles)
		addFiles(files, pkg.EmbedFiles)
		addFiles(files, pkg.IgnoredFiles)
		if pkg.Dir != "" {
			directories[pkg.Dir] = true
			if len(pkg.EmbedPatterns) != 0 {
				recursiveDirectories[pkg.Dir] = true
			}
		}
		imports := make([]string, 0, len(pkg.Imports))
		for path := range pkg.Imports {
			imports = append(imports, path)
		}
		sort.Strings(imports)
		for _, path := range imports {
			writer.addString(path)
			if imported := pkg.Imports[path]; imported != nil {
				writer.addString(imported.ID)
			}
		}
		addModuleFingerprint(writer, pkg.Module, files)
	}
	probe.requiredFiles = mapKeys(files)
	probe.directories = mapKeys(directories)
	probe.recursiveDirectories = mapKeys(recursiveDirectories)
	probe.packageDirectories = directories
}

func (probe *analysisProbe) collectConfigurationFiles() error {
	values := make(map[string]string)
	if err := json.Unmarshal(probe.resolvedEnvironment, &values); err != nil {
		return fmt.Errorf("decode Go build environment: %w", err)
	}
	optional := make(map[string]bool)
	for _, variable := range []string{"GOENV", "GOMOD", "GOWORK"} {
		value := values[variable]
		if value != "" && value != "off" && value != os.DevNull {
			optional[value] = true
		}
	}
	if module := values["GOMOD"]; module != "" && module != os.DevNull {
		moduleRoot := filepath.Dir(module)
		optional[filepath.Join(moduleRoot, "go.sum")] = true
		optional[filepath.Join(moduleRoot, "vendor", "modules.txt")] = true
	}
	if work := values["GOWORK"]; work != "" && work != "off" {
		workRoot := filepath.Dir(work)
		optional[filepath.Join(workRoot, "go.work.sum")] = true
		optional[filepath.Join(workRoot, "vendor", "modules.txt")] = true
	}
	for directory := probe.scope.cwd; ; directory = filepath.Dir(directory) {
		optional[filepath.Join(directory, "go.mod")] = true
		optional[filepath.Join(directory, "go.work")] = true
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
	}
	probe.optionalFiles = mapKeys(optional)
	return nil
}

func (probe *analysisProbe) normalize() {
	sort.Strings(probe.requiredFiles)
	sort.Strings(probe.optionalFiles)
	sort.Strings(probe.directories)
	sort.Strings(probe.recursiveDirectories)
}

func (probe *analysisProbe) currentKey() (analysisCacheKey, error) {
	writer := newFingerprintWriter()
	writer.addString(probeFingerprintVersion)
	writer.addBytes(probe.scope.key[:])
	writer.addBytes(probe.static[:])
	environment := append([]string(nil), os.Environ()...)
	sort.Strings(environment)
	writer.addStrings(environment)
	writer.addBytes(probe.resolvedEnvironment)
	for _, filename := range probe.requiredFiles {
		if err := writer.addFile(filename); err != nil {
			return analysisCacheKey{}, err
		}
	}
	for _, filename := range probe.optionalFiles {
		if err := writer.addOptionalFile(filename); err != nil {
			return analysisCacheKey{}, err
		}
	}
	for _, directory := range probe.directories {
		if err := addDirectoryFingerprint(writer, directory, false); err != nil {
			return analysisCacheKey{}, err
		}
	}
	for _, directory := range probe.recursiveDirectories {
		if err := addDirectoryFingerprint(writer, directory, true); err != nil {
			return analysisCacheKey{}, err
		}
	}
	for _, root := range probe.scope.recursiveRoots {
		if err := addRecursiveTargetTopology(writer, root, probe.packageDirectories); err != nil {
			return analysisCacheKey{}, err
		}
	}
	return writer.sum(), nil
}

func addRecursiveTargetTopology(writer *fingerprintWriter, root string, packageDirectories map[string]bool) error {
	writer.addString(root)
	err := filepath.WalkDir(
		root,
		func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if path == root {
					return nil
				}
				if skippedTopologyDirectory(entry.Name()) {
					return filepath.SkipDir
				}
				boundary := filepath.Join(path, "go.mod")
				info,
					err := os.Stat(boundary)
				switch {
				case err == nil && info.Mode().IsRegular():
					writer.addString("nested-module-boundary")
					if err := writer.addFile(boundary); err != nil {
						return err
					}
					return filepath.SkipDir
				case err == nil:
					return nil
				case os.IsNotExist(err):
					return nil
				default:
					return err
				}
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if entry.Name() == "go.mod" {
				writer.addString(path)
				return nil
			}
			if filepath.Ext(entry.Name()) != ".go" {
				return nil
			}
			writer.addString(path)
			if packageDirectories[filepath.Dir(path)] {
				return nil
			}
			return writer.addFile(path)
		},
	)
	if err != nil {
		return fmt.Errorf("fingerprint recursive target %s: %w", root, err)
	}
	writer.addString("recursive-target-end")
	return nil
}

func skippedTopologyDirectory(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "vendor" || name == "testdata"
}

func addDirectoryFingerprint(writer *fingerprintWriter, directory string, recursive bool) error {
	writer.addString(directory)
	if !recursive {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return fmt.Errorf("fingerprint directory %s: %w", directory, err)
		}
		writer.addUint64(uint64(len(entries)))
		for _, entry := range entries {
			writer.addString(entry.Name())
			writer.addString(entry.Type().String())
		}
		return nil
	}
	type directoryEntry struct {
		path string
		mode fs.FileMode
	}
	entries := make([]directoryEntry, 0)
	err := filepath.WalkDir(
		directory,
		func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path != directory && entry.IsDir() && (entry.Name() == ".git" || strings.HasPrefix(entry.Name(), ".strider-")) {
				return filepath.SkipDir
			}
			relative,
				err := filepath.Rel(directory, path)
			if err != nil {
				return err
			}
			entries = append(entries, directoryEntry{path: relative, mode: entry.Type()})
			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("fingerprint directory %s: %w", directory, err)
	}
	writer.addUint64(uint64(len(entries)))
	for _, entry := range entries {
		writer.addString(entry.path)
		writer.addString(entry.mode.String())
	}
	return nil
}

func mapKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	return result
}
