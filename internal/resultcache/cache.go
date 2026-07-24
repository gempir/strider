// Package resultcache persists path-neutral file-local analysis results.
//
//strider:ignore-file cognitive-complexity,cyclomatic-complexity,no-package-var,top-level-declaration-order,use-errors-new,use-slices-sort
package resultcache

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gempir/strider/internal/buildidentity"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/telemetry"
)

const (
	SchemaVersion   = 1
	defaultMaxBytes = 512 << 20
)

type Options struct {
	Directory string
	MaxBytes  int64
}

// Finding is a diagnostic before invocation-specific path, position, and
// severity materialization.
type Finding struct {
	Code        string           `json:"code"`
	Message     string           `json:"message"`
	Start       int              `json:"start"`
	End         int              `json:"end"`
	StartLine   int              `json:"start_line"`
	StartColumn int              `json:"start_column"`
	EndLine     int              `json:"end_line"`
	EndColumn   int              `json:"end_column"`
	Notes       []Note           `json:"notes,omitempty"`
	Fixes       []diagnostic.Fix `json:"fixes,omitempty"`
}

type Note struct {
	Message     string `json:"message"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
	StartLine   int    `json:"start_line"`
	StartColumn int    `json:"start_column"`
	EndLine     int    `json:"end_line"`
	EndColumn   int    `json:"end_column"`
}

type Entry struct {
	FormatKnown   bool      `json:"format_known,omitempty"`
	FormatChanged bool      `json:"format_changed,omitempty"`
	FormatIgnored bool      `json:"format_ignored,omitempty"`
	Findings      []Finding `json:"findings,omitempty"`
}

type envelope struct {
	SchemaVersion int   `json:"schema_version"`
	Result        Entry `json:"result"`
}

type Cache struct {
	root      string
	entries   string
	maxBytes  int64
	processMu sync.Mutex
	pendingMu sync.Mutex
	pending   map[string]Entry
	build     string
	disabled  bool
}

type cachedFile struct {
	path    string
	size    int64
	modTime time.Time
}

type positionIndex struct {
	source []byte
	lines  []int
}

// Disabled returns a non-persisting cache with the same safe-miss behavior.
func Disabled() *Cache {
	return &Cache{
		disabled: true,
	}
}

// Open prepares the persistent cache.
func Open(options Options) (*Cache, error) {
	root := options.Directory
	explicit := root != ""
	if root == "" {
		root = os.Getenv("STRIDER_CACHE_DIR")
		explicit = root != ""
	}
	if root == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		root = filepath.Join(userCache, "strider")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	cache := &Cache{
		root:     absolute,
		entries:  filepath.Join(absolute, "entries", fmt.Sprintf("v%d", SchemaVersion)),
		maxBytes: maxBytes,
		pending:  make(map[string]Entry),
		build:    buildidentity.Identity(),
	}
	if err := os.MkdirAll(cache.entries, 0o755); err != nil {
		return openFailure(explicit, err)
	}
	return cache, nil
}

func openFailure(explicit bool, cause error) (*Cache, error) {
	if explicit {
		return nil, cause
	}
	return Disabled(), nil
}

// Key hashes complete cache-key components with length framing, the schema,
// and the exact executable identity.
func (cache *Cache) Key(parts ...[]byte) string {
	if cache == nil || cache.disabled {
		return ""
	}
	hash := sha256.New()
	writePart(hash, []byte(fmt.Sprintf("schema:%d", SchemaVersion)))
	writePart(hash, []byte(cache.build))
	for _, part := range parts {
		writePart(hash, part)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// Get returns a safe miss for absent, corrupt, or incompatible entries.
func (cache *Cache) Get(key string) (Entry, bool) {
	if cache == nil || cache.disabled || !validKey(key) {
		return Entry{}, false
	}
	finish := telemetry.Start("cache.lookup")
	defer finish()
	path := cache.entryPath(key)
	contents, err := os.ReadFile(path)
	if err != nil {
		return Entry{}, false
	}
	var stored envelope
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&stored); err != nil || stored.SchemaVersion != SchemaVersion {
		cache.removeCorrupt(path)
		return Entry{}, false
	}
	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil && !errors.Is(err, os.ErrNotExist) {
		return stored.Result, true
	}
	return stored.Result, true
}

// Put atomically publishes an entry and evicts least-recently-used entries
// until the byte bound is satisfied.
func (cache *Cache) Put(key string, entry Entry) error {
	if cache == nil || cache.disabled || !validKey(key) {
		return nil
	}
	finish := telemetry.Start("cache.store")
	defer finish()
	contents, err := encodeEntry(entry)
	if err != nil {
		return err
	}
	return cache.withLock(func() error {
		if err := cache.putLocked(key, contents); err != nil {
			return err
		}
		return cache.evictLocked()
	})
}

// Store queues an entry for one batch publication at the command boundary.
func (cache *Cache) Store(key string, entry Entry) {
	if cache == nil || cache.disabled || !validKey(key) {
		return
	}
	cache.pendingMu.Lock()
	cache.pending[key] = entry
	cache.pendingMu.Unlock()
}

// Flush atomically publishes every queued entry under one cross-process lock.
func (cache *Cache) Flush() error {
	if cache == nil || cache.disabled {
		return nil
	}
	cache.pendingMu.Lock()
	pending := cache.pending
	cache.pending = make(map[string]Entry)
	cache.pendingMu.Unlock()
	if len(pending) == 0 {
		return nil
	}
	encoded := make(map[string][]byte, len(pending))
	for key, entry := range pending {
		contents, err := encodeEntry(entry)
		if err != nil {
			return err
		}
		encoded[key] = contents
	}
	keys := make([]string, 0, len(encoded))
	for key := range encoded {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return cache.withLock(func() error {
		for _, key := range keys {
			if err := cache.putLocked(key, encoded[key]); err != nil {
				return err
			}
		}
		return cache.evictLocked()
	})
}

// FlushBestEffort preserves command correctness when cache storage is
// unavailable.
func (cache *Cache) FlushBestEffort() {
	if err := cache.Flush(); err != nil {
		return
	}
}

// Clear removes all versioned entries while preserving the cross-process lock.
func (cache *Cache) Clear() error {
	if cache == nil || cache.disabled {
		return nil
	}
	return cache.withLock(func() error {
		if err := os.RemoveAll(filepath.Join(cache.root, "entries")); err != nil {
			return err
		}
		return os.MkdirAll(cache.entries, 0o755)
	})
}

func (cache *Cache) entryPath(key string) string {
	return filepath.Join(cache.entries, key[:2], key+".json")
}

func encodeEntry(entry Entry) ([]byte, error) {
	contents, err := json.Marshal(envelope{
		SchemaVersion: SchemaVersion,
		Result:        entry,
	})
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func (cache *Cache) putLocked(key string, contents []byte) error {
	path := cache.entryPath(key)
	_, statErr := os.Stat(path)
	if statErr == nil {
		now := time.Now()
		return os.Chtimes(path, now, now)
	}
	if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".entry-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	cleanup := func(cause error) error {
		return errors.Join(cause, temporary.Close(), os.Remove(temporaryPath))
	}
	if _, err := temporary.Write(contents); err != nil {
		return cleanup(err)
	}
	if err := temporary.Chmod(0o600); err != nil {
		return cleanup(err)
	}
	if err := temporary.Close(); err != nil {
		return errors.Join(err, os.Remove(temporaryPath))
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return errors.Join(err, os.Remove(temporaryPath))
	}
	return nil
}

func (cache *Cache) withLock(run func() error) error {
	cache.processMu.Lock()
	defer cache.processMu.Unlock()
	if err := os.MkdirAll(cache.root, 0o755); err != nil {
		return err
	}
	lockFile, err := os.OpenFile(filepath.Join(cache.root, "cache.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := lockFileExclusive(lockFile); err != nil {
		return errors.Join(err, lockFile.Close())
	}
	runErr := run()
	unlockErr := unlockFile(lockFile)
	return errors.Join(runErr, unlockErr, lockFile.Close())
}

func (cache *Cache) removeCorrupt(path string) {
	if err := cache.withLock(func() error {
		err := os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}); err != nil {
		return
	}
}

func (cache *Cache) evictLocked() error {
	files := []cachedFile{}
	var total int64
	err := filepath.WalkDir(
		cache.entries,
		func(path string, item os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if item.IsDir() || filepath.Ext(item.Name()) != ".json" {
				return nil
			}
			info, err := item.Info()
			if err != nil {
				return err
			}
			files = append(files, cachedFile{
				path:    path,
				size:    info.Size(),
				modTime: info.ModTime(),
			})
			total += info.Size()
			return nil
		},
	)
	if err != nil {
		return err
	}
	sort.Slice(
		files,
		func(left, right int) bool {
			if !files[left].modTime.Equal(files[right].modTime) {
				return files[left].modTime.Before(files[right].modTime)
			}
			return files[left].path < files[right].path
		},
	)
	for _, file := range files {
		if total <= cache.maxBytes {
			break
		}
		if err := os.Remove(file.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		total -= file.size
	}
	return nil
}

func validKey(key string) bool {
	if len(key) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(key)
	return err == nil && !strings.ContainsAny(key, `/\`)
}

func writePart(hash interface {
	Write([]byte) (int, error)
}, part []byte) {
	var size [8]byte
	binary.LittleEndian.PutUint64(size[:], uint64(len(part)))
	if _, err := hash.Write(size[:]); err != nil {
		panic(err)
	}
	if _, err := hash.Write(part); err != nil {
		panic(err)
	}
}

// FindingsFromDiagnostics strips invocation-specific display and severity
// data before persistence.
func FindingsFromDiagnostics(diagnostics []diagnostic.Diagnostic) []Finding {
	result := make([]Finding, 0, len(diagnostics))
	for _, item := range diagnostics {
		finding := Finding{
			Code:        item.Code,
			Message:     item.Message,
			Start:       item.Start.Offset,
			End:         item.End.Offset,
			StartLine:   item.Start.Line,
			StartColumn: item.Start.Column,
			EndLine:     item.End.Line,
			EndColumn:   item.End.Column,
			Fixes:       append([]diagnostic.Fix(nil), item.Fixes...),
		}
		for _, note := range item.Notes {
			finding.Notes = append(
				finding.Notes,
				Note{
					Message:     note.Message,
					Start:       note.Start.Offset,
					End:         note.End.Offset,
					StartLine:   note.Start.Line,
					StartColumn: note.Start.Column,
					EndLine:     note.End.Line,
					EndColumn:   note.End.Column,
				},
			)
		}
		result = append(result, finding)
	}
	return result
}

// Materialize rebuilds display paths, token positions, and effective
// severities for one invocation.
func Materialize(display string, source []byte, findings []Finding, severity func(string) diagnostic.Severity) []diagnostic.Diagnostic {
	positions := newPositionIndex(source)
	result := make([]diagnostic.Diagnostic, 0, len(findings))
	for _, finding := range findings {
		item := diagnostic.Diagnostic{
			Code:     finding.Code,
			Message:  finding.Message,
			Severity: severity(finding.Code),
			File:     display,
			Start:    positions.position(finding.Start, finding.StartLine, finding.StartColumn, display),
			End:      positions.position(finding.End, finding.EndLine, finding.EndColumn, display),
			Fixes:    append([]diagnostic.Fix(nil), finding.Fixes...),
		}
		for _, note := range finding.Notes {
			item.Notes = append(
				item.Notes,
				diagnostic.Note{
					Message: note.Message,
					Start:   positions.position(note.Start, note.StartLine, note.StartColumn, display),
					End:     positions.position(note.End, note.EndLine, note.EndColumn, display),
				},
			)
		}
		result = append(result, item)
	}
	return result
}

func newPositionIndex(source []byte) positionIndex {
	lines := make([]int, 0, len(source)/40+1)
	lines = append(lines, 0)
	for index, current := range source {
		if current == '\n' {
			lines = append(lines, index+1)
		}
	}
	return positionIndex{
		source: source,
		lines:  lines,
	}
}

func (index positionIndex) position(offset, cachedLine, cachedColumn int, display string) token.Position {
	boundedOffset := max(0, min(offset, len(index.source)))
	if cachedLine > 0 && cachedColumn > 0 {
		return token.Position{
			Filename: display,
			Offset:   boundedOffset,
			Line:     cachedLine,
			Column:   cachedColumn,
		}
	}
	line := sort.Search(len(index.lines), func(line int) bool {
		return index.lines[line] > boundedOffset
	}) - 1
	line = max(line, 0)
	return token.Position{
		Filename: display,
		Offset:   boundedOffset,
		Line:     line + 1,
		Column:   boundedOffset - index.lines[line] + 1,
	}
}
