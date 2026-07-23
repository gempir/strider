package checks

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/gempir/strider/internal/checks/semantic"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/workspace"
)

// SessionStats reports concrete-syntax reuse across completed workspace
// generations. Package-aware checks intentionally run fresh each time.
type SessionStats struct {
	ConcreteHits   uint64
	ConcreteMisses uint64
}

// Session executes a fixed check registry and option set across immutable
// workspace generations. Unchanged source snapshots reuse concrete findings;
// package-aware checks run fresh so watch mode does not need a second package
// load merely to prove a cached result reusable.
type Session struct {
	mu sync.Mutex

	registry *Registry
	options  RunOptions

	hasConcrete bool
	concreteKey [sha256.Size]byte
	concrete    Result
	hits        uint64
	misses      uint64
}

type moduleFileState struct {
	exists   bool
	contents []byte
}

// NewSession constructs a bounded incremental check session. The registry and
// run options are immutable for the lifetime of the session.
func NewSession(registry *Registry, options RunOptions) (*Session, error) {
	if registry == nil {
		return nil, fmt.Errorf("check session registry is nil")
	}
	if options.Formatter.PrintWidth == 0 {
		options.Formatter = formatter.DefaultOptions()
	}
	return &Session{
		registry: registry,
		options:  options,
	}, nil
}

// Run checks one immutable workspace generation and returns an owned result.
// Calls are serialized to coalesce identical generations and keep publication
// of cached concrete findings deterministic.
func (session *Session) Run(ctx context.Context, shared *workspace.Workspace) (Result, error) {
	if session == nil {
		return Result{}, fmt.Errorf("run check session: nil session")
	}
	if shared == nil {
		return Result{}, fmt.Errorf("run check session: nil workspace")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	session.mu.Lock()
	defer session.mu.Unlock()

	key, err := concreteFingerprint(shared, session.registry.format)
	if err != nil {
		return Result{}, err
	}
	var result Result
	if session.hasConcrete && key == session.concreteKey {
		session.hits++
		result = cloneResult(session.concrete)
	} else {
		session.misses++
		result, err = runConcreteChecks(ctx, shared.Files(), session.registry, session.options.Formatter, session.options.CollectCandidates)
		if err != nil {
			return Result{}, err
		}
		session.concreteKey = key
		session.concrete = cloneResult(result)
		session.hasConcrete = true
	}
	if err := appendAnalysis(ctx, &result, shared, session.registry, semantic.RunContext); err != nil {
		return Result{}, err
	}
	filterExcludedResults(&result, session.options.Root, session.options.Excludes)
	diagnostic.Sort(result.Diagnostics)
	if result.Diagnostics == nil {
		result.Diagnostics = []diagnostic.Diagnostic{}
	}
	return result, nil
}

// Stats returns point-in-time concrete and package-aware cache counters.
func (session *Session) Stats() SessionStats {
	if session == nil {
		return SessionStats{}
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return SessionStats{
		ConcreteHits:   session.hits,
		ConcreteMisses: session.misses,
	}
}

func concreteFingerprint(shared *workspace.Workspace, includeFormatterContext bool) ([sha256.Size]byte, error) {
	digest := sha256.New()
	files := shared.Files()
	writeUint64(digest, uint64(len(files)))
	for _, file := range files {
		writeBytes(digest, []byte(file.Path()))
		identity, err := file.Identity()
		if err != nil {
			return [sha256.Size]byte{}, err
		}
		_, _ = digest.Write(identity[:])
	}
	if includeFormatterContext {
		addFormatterModuleFingerprint(digest, files)
	}
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result, nil
}

// addFormatterModuleFingerprint mirrors modulePathCache.find: formatting can
// group imports differently when the nearest go.mod changes, even if no Go
// source changed. Missing candidates are included so creating a closer module
// boundary also invalidates an incremental concrete result.
func addFormatterModuleFingerprint(writer hash.Hash, files []*workspace.File) {
	states := make(map[string]moduleFileState)
	for _, file := range files {
		for directory := filepath.Dir(file.Path()); ; directory = filepath.Dir(directory) {
			path := filepath.Join(directory, "go.mod")
			state, seen := states[path]
			if !seen {
				contents, err := os.ReadFile(path)
				state = moduleFileState{
					exists:   err == nil,
					contents: contents,
				}
				states[path] = state
			}
			if state.exists {
				break
			}
			parent := filepath.Dir(directory)
			if parent == directory {
				break
			}
		}
	}
	paths := make([]string, 0, len(states))
	for path := range states {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	writeUint64(writer, uint64(len(paths)))
	for _, path := range paths {
		writeBytes(writer, []byte(path))
		state := states[path]
		if !state.exists {
			writeUint64(writer, 0)
			continue
		}
		writeUint64(writer, 1)
		writeBytes(writer, state.contents)
	}
}

func writeBytes(writer hash.Hash, value []byte) {
	writeUint64(writer, uint64(len(value)))
	_, _ = writer.Write(value)
}

func writeUint64(writer hash.Hash, value uint64) {
	var encoded [8]byte
	binary.LittleEndian.PutUint64(encoded[:], value)
	_, _ = writer.Write(encoded[:])
}

func cloneResult(original Result) Result {
	result := Result{
		Diagnostics: cloneDiagnostics(original.Diagnostics),
	}
	if original.Candidates != nil {
		result.Candidates = make(map[string]formatter.Result, len(original.Candidates))
		for filename, candidate := range original.Candidates {
			candidate.Source = append([]byte(nil), candidate.Source...)
			result.Candidates[filename] = candidate
		}
	}
	return result
}

func cloneDiagnostics(original []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	if original == nil {
		return nil
	}
	result := append([]diagnostic.Diagnostic(nil), original...)
	for index := range result {
		result[index].Notes = append([]diagnostic.Note(nil), original[index].Notes...)
		result[index].Fixes = append([]diagnostic.Fix(nil), original[index].Fixes...)
		for fixIndex := range result[index].Fixes {
			result[index].Fixes[fixIndex].Edits = append([]diagnostic.TextEdit(nil), original[index].Fixes[fixIndex].Edits...)
		}
	}
	return result
}
