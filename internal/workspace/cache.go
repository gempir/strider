package workspace

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/pathfilter"
	"github.com/gempir/strider/internal/source"
)

const (
	defaultCacheEntries   = 2048
	defaultCacheBytes     = 256 << 20
	defaultCacheTreeBytes = 256 << 20
	// modernc's pointer-rich CST can be much larger than its source. The
	// estimate intentionally favors a bounded watch heap over retaining every
	// parsed file; live generations remain valid after cache eviction.
	cstEstimateMultiplier = 128
	cstEstimateFloor      = 128 << 10
)

// ContentIdentity is a collision-resistant identity for one complete source
// snapshot. The file path is deliberately not part of the digest; Cache keys
// add it because CSTs retain their parse filename.
type ContentIdentity [sha256.Size]byte

// CacheOptions bounds snapshots retained between immutable workspace
// generations. MaxBytes counts retained source bytes exactly. MaxTreeBytes
// bounds a conservative estimate of retained CST heap, and MaxEntries bounds
// snapshots independently. Non-positive values select conservative defaults.
type CacheOptions struct {
	MaxEntries   int
	MaxBytes     int64
	MaxTreeBytes int64
}

// CacheStats is a point-in-time view of cache use. Hits and misses count files
// considered while successfully opening generations. Bytes is exact retained
// source size; TreeBytes is the conservative CST estimate used for eviction.
type CacheStats struct {
	Generations uint64
	Hits        uint64
	Misses      uint64
	Evictions   uint64
	Entries     int
	Bytes       int64
	TreeBytes   int64
}

// Cache creates immutable workspace generations and shares unchanged source
// snapshots and their lazily parsed CSTs between them. Open calls are
// serialized so generation publication and LRU order are deterministic.
type Cache struct {
	openMu       sync.Mutex
	mu           sync.Mutex
	maxEntries   int
	maxBytes     int64
	maxTreeBytes int64
	generation   uint64
	clock        uint64
	entries      map[cacheKey]*cacheEntry
	bytes        int64
	treeBytes    int64
	hits         uint64
	misses       uint64
	evictions    uint64
}

type cacheKey struct {
	path     string
	identity ContentIdentity
}

type cacheEntry struct {
	snapshot  *fileSnapshot
	lastUsed  uint64
	treeBytes int64
}

type fileSnapshot struct {
	path     string
	identity ContentIdentity
	source   []byte
	treeOnce sync.Once
	tree     *cst.Tree
	treeErr  error
	onTree   func(int64)
}

func (snapshot *fileSnapshot) CST() (*cst.Tree, error) {
	snapshot.treeOnce.Do(
		func() {
			snapshot.tree,
				snapshot.treeErr = cst.Parse(snapshot.path, snapshot.source)
			if snapshot.tree != nil && snapshot.onTree != nil {
				snapshot.onTree(estimatedCSTBytes(snapshot.source))
			}
		},
	)
	return snapshot.tree, snapshot.treeErr
}

// NewCache constructs a bounded in-memory generation cache.
func NewCache(options CacheOptions) *Cache {
	maxEntries := options.MaxEntries
	if maxEntries <= 0 {
		maxEntries = defaultCacheEntries
	}
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultCacheBytes
	}
	maxTreeBytes := options.MaxTreeBytes
	if maxTreeBytes <= 0 {
		maxTreeBytes = defaultCacheTreeBytes
	}
	return &Cache{
		maxEntries:   maxEntries,
		maxBytes:     maxBytes,
		maxTreeBytes: maxTreeBytes,
		entries:      make(map[cacheKey]*cacheEntry),
	}
}

// Open captures a complete immutable workspace generation. Each discovered
// file is read and hashed before the generation is published. Generation-local
// File handles may be released without affecting snapshots retained for a
// later generation.
func (cache *Cache) Open(paths []string, options Options) (*Workspace, error) {
	if cache == nil {
		return nil, fmt.Errorf("open cached workspace: nil cache")
	}
	cache.openMu.Lock()
	defer cache.openMu.Unlock()

	inputs := append([]string(nil), paths...)
	if len(inputs) == 0 {
		inputs = []string{
			".",
		}
	}
	filenames, err := source.Discover(inputs, source.Options{})
	if err != nil {
		return nil, err
	}
	type capturedFile struct {
		path     string
		contents []byte
		identity ContentIdentity
	}
	captured := make([]capturedFile, 0, len(filenames))
	for _, filename := range filenames {
		if pathfilter.Excluded(options.Root, filename, options.Excludes) {
			continue
		}
		contents, readErr := os.ReadFile(filename)
		if readErr != nil {
			return nil, fmt.Errorf("read workspace file %s: %w", filename, readErr)
		}
		if options.SkipGenerated && generatedSource(contents) {
			continue
		}
		captured = append(captured, capturedFile{
			path:     filename,
			contents: contents,
			identity: sha256.Sum256(contents),
		})
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.generation++
	files := make([]*File, 0, len(captured))
	for _, item := range captured {
		key := cacheKey{
			path:     item.path,
			identity: item.identity,
		}
		entry := cache.entries[key]
		if entry == nil {
			cache.misses++
			snapshot := &fileSnapshot{
				path:     item.path,
				identity: item.identity,
				source:   item.contents,
			}
			snapshot.onTree = func(treeBytes int64) {
				cache.recordTree(key, snapshot, treeBytes)
			}
			entry = &cacheEntry{
				snapshot: snapshot,
			}
			if int64(len(item.contents)) <= cache.maxBytes {
				cache.entries[key] = entry
				cache.bytes += int64(len(item.contents))
			}
		} else {
			cache.hits++
		}
		cache.clock++
		entry.lastUsed = cache.clock
		files = append(files, &File{
			path:     item.path,
			snapshot: entry.snapshot,
		})
	}
	cache.evictLocked()
	return &Workspace{
		inputs: inputs,
		files:  files,
	}, nil
}

// Stats returns cache counters and current retained resource use.
func (cache *Cache) Stats() CacheStats {
	if cache == nil {
		return CacheStats{}
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return CacheStats{
		Generations: cache.generation,
		Hits:        cache.hits,
		Misses:      cache.misses,
		Evictions:   cache.evictions,
		Entries:     len(cache.entries),
		Bytes:       cache.bytes,
		TreeBytes:   cache.treeBytes,
	}
}

// Invalidate removes all snapshots retained for later generations. Existing
// workspaces remain valid because they own immutable snapshot references.
func (cache *Cache) Invalidate() {
	if cache == nil {
		return
	}
	cache.openMu.Lock()
	defer cache.openMu.Unlock()
	cache.mu.Lock()
	cache.entries = make(map[cacheKey]*cacheEntry)
	cache.bytes = 0
	cache.treeBytes = 0
	cache.mu.Unlock()
}

func (cache *Cache) evictLocked() {
	for len(cache.entries) > cache.maxEntries || cache.bytes > cache.maxBytes || cache.treeBytes > cache.maxTreeBytes {
		treePressure := cache.treeBytes > cache.maxTreeBytes
		keys := make([]cacheKey, 0, len(cache.entries))
		for key, entry := range cache.entries {
			if treePressure && entry.treeBytes == 0 {
				continue
			}
			keys = append(keys, key)
		}
		sort.Slice(
			keys,
			func(leftIndex, rightIndex int) bool {
				left := cache.entries[keys[leftIndex]]
				right := cache.entries[keys[rightIndex]]
				if left.lastUsed != right.lastUsed {
					return left.lastUsed < right.lastUsed
				}
				if keys[leftIndex].path != keys[rightIndex].path {
					return keys[leftIndex].path < keys[rightIndex].path
				}
				return bytes.Compare(keys[leftIndex].identity[:], keys[rightIndex].identity[:]) < 0
			},
		)
		key := keys[0]
		cache.bytes -= int64(len(cache.entries[key].snapshot.source))
		cache.treeBytes -= cache.entries[key].treeBytes
		delete(cache.entries, key)
		cache.evictions++
	}
}

func (cache *Cache) recordTree(key cacheKey, snapshot *fileSnapshot, treeBytes int64) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	entry := cache.entries[key]
	if entry == nil || entry.snapshot != snapshot || entry.treeBytes != 0 {
		return
	}
	entry.treeBytes = treeBytes
	cache.treeBytes += treeBytes
	cache.evictLocked()
}

func estimatedCSTBytes(source []byte) int64 {
	estimate := int64(len(source)) * cstEstimateMultiplier
	if estimate < cstEstimateFloor {
		return cstEstimateFloor
	}
	return estimate
}

func generatedSource(contents []byte) bool {
	limited := contents
	if len(limited) > 4096 {
		limited = limited[:4096]
	}
	scanner := bufio.NewScanner(bytes.NewReader(limited))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if bytes.HasPrefix(line, []byte("// Code generated ")) && bytes.HasSuffix(line, []byte(" DO NOT EDIT.")) {
			return true
		}
	}
	return false
}
