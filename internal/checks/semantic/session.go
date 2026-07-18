package semantic

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"

	"github.com/gempir/strider/internal/diagnostic"
)

const (
	defaultSessionEntries = 8
	defaultSessionBytes   = 16 << 20
	fingerprintVersion    = "strider-analyze-session-v1"
	maxGenerationAttempts = 3
)

const fingerprintLoadMode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps | packages.NeedModule | packages.NeedEmbedFiles

var resolvedGoEnvironment = []string{
	"GOOS",
	"GOARCH",
	"GO386",
	"GOAMD64",
	"GOARM",
	"GOARM64",
	"GOMIPS",
	"GOMIPS64",
	"GOPPC64",
	"GORISCV64",
	"GOWASM",
	"CGO_ENABLED",
	"CC",
	"CXX",
	"PKG_CONFIG",
	"GOFLAGS",
	"GOEXPERIMENT",
	"GOTOOLCHAIN",
	"GOROOT",
	"GOPATH",
	"GOMOD",
	"GOWORK",
	"GOENV",
	"GOVERSION",
}

// SessionOptions bounds completed whole-target results retained across watch
// iterations. MaxBytes is a deterministic estimate of the diagnostic payload
// owned by the cache. Non-positive values select conservative defaults.
type SessionOptions struct {
	MaxEntries int
	MaxBytes   int64
}

// SessionStats is a point-in-time view of whole-target result reuse.
// Generation advances when a newly verified target fingerprint is published,
// even if its diagnostic payload is too large to retain in the result cache.
type SessionStats struct {
	Generation uint64
	Hits       uint64
	Misses     uint64
	Evictions  uint64
	Entries    int
	Bytes      int64
}

// Session safely reuses completed analysis results only when a fresh metadata
// fingerprint proves the entire target equivalent. It never stores AST,
// types, facts, or SSA pointers. Calls are serialized because a watch session
// has one logical sequence of package generations and this also coalesces
// concurrent requests for an identical generation.
type Session struct {
	runMu            sync.Mutex
	mu               sync.Mutex
	maxEntries       int
	maxBytes         int64
	clock            uint64
	entries          map[analysisCacheKey]*analysisCacheEntry
	bytes            int64
	hits             uint64
	misses           uint64
	evictions        uint64
	generation       uint64
	stableKey        analysisCacheKey
	hasStableKey     bool
	fingerprint      sessionFingerprintFunc
	resetFingerprint func()
	runner           sessionRunFunc
}

type analysisCacheKey [sha256.Size]byte

type analysisCacheEntry struct {
	diagnostics []diagnostic.Diagnostic
	bytes       int64
	lastUsed    uint64
}

type sessionFingerprintFunc func([]string, *Registry) (analysisCacheKey, error)

type sessionRunFunc func([]string, *Registry) ([]diagnostic.Diagnostic, error)

// NewSession constructs a bounded, memory-only analysis session.
func NewSession(options SessionOptions) *Session {
	fingerprinter := &analysisSessionFingerprinter{}
	session := newSession(options, fingerprinter.fingerprint, Run)
	session.resetFingerprint = fingerprinter.reset
	return session
}

func newSession(options SessionOptions, fingerprint sessionFingerprintFunc, runner sessionRunFunc) *Session {
	maxEntries := options.MaxEntries
	if maxEntries <= 0 {
		maxEntries = defaultSessionEntries
	}
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultSessionBytes
	}
	return &Session{maxEntries: maxEntries, maxBytes: maxBytes, entries: make(map[analysisCacheKey]*analysisCacheEntry), fingerprint: fingerprint, runner: runner}
}

// Run analyzes paths or returns a deep copy of a proven-equivalent cached
// result. Failed loads and failed analyses are never cached.
func (session *Session) Run(paths []string, registry *Registry) ([]diagnostic.Diagnostic, error) {
	if session == nil {
		return nil, fmt.Errorf("run analysis session: nil session")
	}
	if registry == nil {
		return nil, fmt.Errorf("run analysis session: nil registry")
	}
	session.runMu.Lock()
	defer session.runMu.Unlock()
	for range maxGenerationAttempts {
		key, err := session.fingerprint(paths, registry)
		if err != nil {
			return nil, err
		}
		session.mu.Lock()
		entry := session.entries[key]
		session.mu.Unlock()
		if entry != nil {
			confirmed, confirmErr := session.fingerprint(paths, registry)
			if confirmErr != nil {
				return nil, confirmErr
			}
			if confirmed != key {
				continue
			}
			session.mu.Lock()
			session.publishGenerationLocked(key)
			session.hits++
			session.clock++
			entry.lastUsed = session.clock
			diagnostics := cloneDiagnostics(entry.diagnostics)
			session.mu.Unlock()
			return diagnostics, nil
		}

		session.mu.Lock()
		session.misses++
		session.mu.Unlock()
		diagnostics, err := session.runner(paths, registry)
		if err != nil {
			return nil, err
		}
		confirmed, err := session.fingerprint(paths, registry)
		if err != nil {
			return nil, err
		}
		if confirmed != key {
			continue
		}
		owned := cloneDiagnostics(diagnostics)
		weight := diagnosticPayloadBytes(owned)
		session.mu.Lock()
		session.publishGenerationLocked(key)
		if weight <= session.maxBytes {
			session.clock++
			session.entries[key] = &analysisCacheEntry{diagnostics: owned, bytes: weight, lastUsed: session.clock}
			session.bytes += weight
			session.evictLocked()
		}
		session.mu.Unlock()
		return cloneDiagnostics(diagnostics), nil
	}
	return nil, fmt.Errorf("analyze inputs changed during %d consecutive attempts", maxGenerationAttempts)
}

// Stats returns cache counters and current retained result payload.
func (session *Session) Stats() SessionStats {
	if session == nil {
		return SessionStats{}
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return SessionStats{Generation: session.generation, Hits: session.hits, Misses: session.misses, Evictions: session.evictions, Entries: len(session.entries), Bytes: session.bytes}
}

// Invalidate drops all completed results. An in-flight Run finishes before
// invalidation so it cannot repopulate an explicitly cleared session.
func (session *Session) Invalidate() {
	if session == nil {
		return
	}
	session.runMu.Lock()
	defer session.runMu.Unlock()
	session.mu.Lock()
	session.entries = make(map[analysisCacheKey]*analysisCacheEntry)
	session.bytes = 0
	session.stableKey = analysisCacheKey{}
	session.hasStableKey = false
	session.mu.Unlock()
	if session.resetFingerprint != nil {
		session.resetFingerprint()
	}
}

func (session *Session) publishGenerationLocked(key analysisCacheKey) {
	if session.hasStableKey && session.stableKey == key {
		return
	}
	session.generation++
	session.stableKey = key
	session.hasStableKey = true
}

func (session *Session) evictLocked() {
	for len(session.entries) > session.maxEntries || session.bytes > session.maxBytes {
		keys := make([]analysisCacheKey, 0, len(session.entries))
		for key := range session.entries {
			keys = append(keys, key)
		}
		sort.Slice(
			keys,
			func(leftIndex, rightIndex int) bool {
				left := session.entries[keys[leftIndex]]
				right := session.entries[keys[rightIndex]]
				if left.lastUsed != right.lastUsed {
					return left.lastUsed < right.lastUsed
				}
				return strings.Compare(string(keys[leftIndex][:]), string(keys[rightIndex][:])) < 0
			},
		)
		key := keys[0]
		session.bytes -= session.entries[key].bytes
		delete(session.entries, key)
		session.evictions++
	}
}

func analysisFingerprint(paths []string, registry *Registry) (analysisCacheKey, error) {
	if registry == nil {
		return analysisCacheKey{}, fmt.Errorf("fingerprint analysis: nil registry")
	}
	patterns, _, err := loadInputs(paths)
	if err != nil {
		return analysisCacheKey{}, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return analysisCacheKey{}, err
	}
	if cwd, err = canonicalPath(cwd); err != nil {
		return analysisCacheKey{}, err
	}
	environment, resolved, err := analysisEnvironment(cwd)
	if err != nil {
		return analysisCacheKey{}, err
	}
	loaded, err := packages.Load(&packages.Config{Mode: fingerprintLoadMode, Tests: true}, patterns...)
	if err != nil {
		return analysisCacheKey{}, err
	}
	if err := packageError(loaded); err != nil {
		return analysisCacheKey{}, err
	}

	writer := newFingerprintWriter()
	writer.addString(fingerprintVersion)
	writer.addString(runtime.Version())
	writer.addString(cwd)
	effectivePaths := paths
	if len(effectivePaths) == 0 {
		effectivePaths = []string{"."}
	}
	writer.addStrings(effectivePaths)
	writer.addStrings(patterns)
	writer.addStrings(environment)
	writer.addBytes(resolved)
	addRegistryFingerprint(writer, registry)
	if err := addPackageGraphFingerprint(writer, loaded); err != nil {
		return analysisCacheKey{}, err
	}
	if err := addGoConfigurationFiles(writer, resolved); err != nil {
		return analysisCacheKey{}, err
	}
	return writer.sum(), nil
}

func analysisEnvironment(cwd string) ([]string, []byte, error) {
	environment := append([]string(nil), os.Environ()...)
	sort.Strings(environment)
	arguments := append([]string{"env", "-json"}, resolvedGoEnvironment...)
	command := exec.Command("go", arguments...)
	command.Dir = cwd
	resolved, err := command.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, nil, fmt.Errorf("resolve Go build environment: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, nil, fmt.Errorf("resolve Go build environment: %w", err)
	}
	return environment, resolved, nil
}

func addRegistryFingerprint(writer *fingerprintWriter, registry *Registry) {
	type ruleFingerprint struct {
		code     string
		severity string
		excludes []string
	}
	rules := make([]ruleFingerprint, 0, len(registry.rules))
	for _, rule := range registry.rules {
		code := rule.Meta().Code
		setting := registry.settings[code]
		excludes := append([]string(nil), setting.excludes...)
		sort.Strings(excludes)
		rules = append(rules, ruleFingerprint{code: code, severity: string(setting.severity), excludes: excludes})
	}
	sort.Slice(rules, func(leftIndex, rightIndex int) bool {
		return rules[leftIndex].code < rules[rightIndex].code
	})
	writer.addString(registry.root)
	for _, rule := range rules {
		writer.addString(rule.code)
		writer.addString(rule.severity)
		writer.addStrings(rule.excludes)
	}
	plan := registry.executionPlan()
	writer.addUint64(uint64(plan.requirements.Stage))
	writer.addUint64(uint64(plan.requirements.Facts))
	writer.addUint64(uint64(plan.requirements.SSAFeatures))
	writer.addUint64(uint64(plan.ssaBuilderMode()))
}

func addPackageGraphFingerprint(writer *fingerprintWriter, roots []*packages.Package) error {
	packagesByPointer := make(map[*packages.Package]bool)
	var visit func(*packages.Package)
	visit = func(pkg *packages.Package) {
		if pkg == nil || packagesByPointer[pkg] {
			return
		}
		packagesByPointer[pkg] = true
		for _, imported := range pkg.Imports {
			visit(imported)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	all := make([]*packages.Package, 0, len(packagesByPointer))
	for pkg := range packagesByPointer {
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
	filenames := make([]string, 0, len(files))
	for filename := range files {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)
	for _, filename := range filenames {
		if err := writer.addFile(filename); err != nil {
			return err
		}
	}
	return nil
}

func addModuleFingerprint(writer *fingerprintWriter, module *packages.Module, files map[string]bool) {
	for module != nil {
		writer.addString(module.Path)
		writer.addString(module.Version)
		writer.addString(module.Dir)
		writer.addString(module.GoMod)
		writer.addString(module.GoVersion)
		if module.Main {
			writer.addString("main")
		}
		if module.Indirect {
			writer.addString("indirect")
		}
		if module.Error != nil {
			writer.addString(module.Error.Err)
		}
		if module.GoMod != "" {
			files[module.GoMod] = true
		}
		module = module.Replace
	}
}

func addFiles(set map[string]bool, filenames []string) {
	for _, filename := range filenames {
		if filename != "" {
			set[filename] = true
		}
	}
}

func addGoConfigurationFiles(writer *fingerprintWriter, resolved []byte) error {
	values := make(map[string]string)
	if err := json.Unmarshal(resolved, &values); err != nil {
		return fmt.Errorf("decode Go build environment: %w", err)
	}
	files := make(map[string]bool)
	for _, variable := range []string{"GOENV", "GOMOD", "GOWORK"} {
		value := values[variable]
		if value != "" && value != "off" && value != os.DevNull {
			files[value] = true
		}
	}
	if module := values["GOMOD"]; module != "" && module != os.DevNull {
		files[filepath.Join(filepath.Dir(module), "go.sum")] = true
	}
	if work := values["GOWORK"]; work != "" && work != "off" {
		files[filepath.Join(filepath.Dir(work), "go.work.sum")] = true
	}
	filenames := make([]string, 0, len(files))
	for filename := range files {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)
	for _, filename := range filenames {
		if err := writer.addOptionalFile(filename); err != nil {
			return err
		}
	}
	return nil
}

type fingerprintWriter struct {
	hash hash.Hash
}

func newFingerprintWriter() *fingerprintWriter {
	return &fingerprintWriter{hash: sha256.New()}
}

func (writer *fingerprintWriter) addUint64(value uint64) {
	var encoded [8]byte
	binary.LittleEndian.PutUint64(encoded[:], value)
	_, _ = writer.hash.Write(encoded[:])
}

func (writer *fingerprintWriter) addBytes(value []byte) {
	writer.addUint64(uint64(len(value)))
	_, _ = writer.hash.Write(value)
}

func (writer *fingerprintWriter) addString(value string) {
	writer.addBytes([]byte(value))
}

func (writer *fingerprintWriter) addStrings(values []string) {
	writer.addUint64(uint64(len(values)))
	for _, value := range values {
		writer.addString(value)
	}
}

func (writer *fingerprintWriter) addFile(filename string) error {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("fingerprint %s: %w", filename, err)
	}
	writer.addString(filename)
	writer.addBytes(contents)
	return nil
}

func (writer *fingerprintWriter) addOptionalFile(filename string) error {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return writer.addOptionalFileError(filename, err)
	}
	writer.addString(filename)
	writer.addBytes(contents)
	return nil
}

func (writer *fingerprintWriter) addOptionalFileError(filename string, readErr error) error {
	if !os.IsNotExist(readErr) {
		return fmt.Errorf("fingerprint %s: %w", filename, readErr)
	}
	writer.addString(filename)
	writer.addString("missing")
	return nil
}

func (writer *fingerprintWriter) sum() analysisCacheKey {
	var result analysisCacheKey
	copy(result[:], writer.hash.Sum(nil))
	return result
}

func cloneDiagnostics(sourceDiagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	if sourceDiagnostics == nil {
		return nil
	}
	result := make([]diagnostic.Diagnostic, len(sourceDiagnostics))
	for index, item := range sourceDiagnostics {
		result[index] = item
		result[index].Notes = append([]diagnostic.Note(nil), item.Notes...)
		result[index].Fixes = make([]diagnostic.Fix, len(item.Fixes))
		for fixIndex, fix := range item.Fixes {
			result[index].Fixes[fixIndex] = fix
			result[index].Fixes[fixIndex].Edits = append([]diagnostic.TextEdit(nil), fix.Edits...)
		}
	}
	return result
}

func diagnosticPayloadBytes(diagnostics []diagnostic.Diagnostic) int64 {
	var total int64
	for _, item := range diagnostics {
		total += 256
		total += int64(len(item.Code) + len(item.Message) + len(item.File))
		total += int64(len(item.Start.Filename) + len(item.End.Filename))
		for _, note := range item.Notes {
			total += 128
			total += int64(len(note.Message) + len(note.Start.Filename) + len(note.End.Filename))
		}
		for _, fix := range item.Fixes {
			total += 64 + int64(len(fix.Message)+len(fix.Safety))
			for _, edit := range fix.Edits {
				total += 32 + int64(len(edit.NewText))
			}
		}
	}
	return total
}
