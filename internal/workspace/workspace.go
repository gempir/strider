// Package workspace owns source files and their lazily constructed syntax
// representations for one Strider run.
package workspace

import (
	"crypto/sha256"
	"fmt"
	"os"
	"sync"

	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/pathfilter"
	"github.com/gempir/strider/internal/source"
)

// Options controls source discovery for a workspace.
type Options struct {
	SkipGenerated bool
	Root          string
	Excludes      []string
}

// Workspace is an immutable set of input paths and source files. File contents
// and CSTs are loaded lazily and are safe to request concurrently.
type Workspace struct {
	inputs     []string
	files      []*File
	generation uint64
}

// File owns the cached source and CST for one Go file.
type File struct {
	path     string
	snapshot *fileSnapshot

	bytesOnce     sync.Once
	bytesMu       sync.RWMutex
	bytes         []byte
	bytesErr      error
	bytesReleased bool
	treeOnce      sync.Once
	treeMu        sync.RWMutex
	tree          *cst.Tree
	treeErr       error
	treeReleased  bool
}

// Open discovers a deterministic workspace for paths.
func Open(paths []string, options Options) (*Workspace, error) {
	inputs := append([]string(nil), paths...)
	if len(inputs) == 0 {
		inputs = []string{"."}
	}
	filenames, err := source.Discover(inputs, source.Options{SkipGenerated: options.SkipGenerated})
	if err != nil {
		return nil, err
	}
	files := make([]*File, 0, len(filenames))
	for _, filename := range filenames {
		if pathfilter.Matches(options.Root, filename, options.Excludes) {
			continue
		}
		files = append(files, &File{path: filename})
	}
	return &Workspace{inputs: inputs, files: files}, nil
}

// Inputs returns the original file and directory inputs. The returned slice is
// independent from the workspace.
func (workspace *Workspace) Inputs() []string {
	if workspace == nil {
		return nil
	}
	return append([]string(nil), workspace.inputs...)
}

// Files returns the discovered files in deterministic path order. The File
// values remain owned by the workspace and may be shared between engines.
func (workspace *Workspace) Files() []*File {
	if workspace == nil {
		return nil
	}
	return append([]*File(nil), workspace.files...)
}

// Generation returns the cache generation that owns this immutable view. A
// workspace opened through Open is a one-shot generation and returns zero.
func (workspace *Workspace) Generation() uint64 {
	if workspace == nil {
		return 0
	}
	return workspace.generation
}

// Path returns the absolute source filename.
func (file *File) Path() string {
	if file == nil {
		return ""
	}
	return file.path
}

// Bytes returns the cached source. The returned slice is owned by File and
// must be treated as read-only.
func (file *File) Bytes() ([]byte, error) {
	if file == nil {
		return nil, fmt.Errorf("read workspace file: nil file")
	}
	if file.snapshot != nil {
		file.bytesMu.RLock()
		released := file.bytesReleased
		file.bytesMu.RUnlock()
		if released {
			return nil, fmt.Errorf("read workspace file %s: source cache released", file.path)
		}
		return file.snapshot.source, nil
	}
	file.bytesOnce.Do(func() {
		contents, err := os.ReadFile(file.path)
		file.bytesMu.Lock()
		if !file.bytesReleased {
			file.bytes = contents
			file.bytesErr = err
		}
		file.bytesMu.Unlock()
	})
	file.bytesMu.RLock()
	defer file.bytesMu.RUnlock()
	if file.bytesReleased {
		return nil, fmt.Errorf("read workspace file %s: source cache released", file.path)
	}
	return file.bytes, file.bytesErr
}

// CST returns the cached lossless concrete syntax tree.
func (file *File) CST() (*cst.Tree, error) {
	if file == nil {
		return nil, fmt.Errorf("parse workspace file: nil file")
	}
	if file.snapshot != nil {
		file.treeMu.RLock()
		released := file.treeReleased
		file.treeMu.RUnlock()
		if released {
			return nil, fmt.Errorf("parse workspace file %s: CST cache released", file.path)
		}
		tree, err := file.snapshot.CST()
		file.treeMu.RLock()
		released = file.treeReleased
		file.treeMu.RUnlock()
		if released {
			return nil, fmt.Errorf("parse workspace file %s: CST cache released", file.path)
		}
		return tree, err
	}
	file.treeOnce.Do(
		func() {
			contents,
				err := file.Bytes()
			if err != nil {
				file.treeMu.Lock()
				file.treeErr = err
				file.treeMu.Unlock()
				return
			}
			tree,
				parseErr := cst.Parse(file.path, contents)
			file.treeMu.Lock()
			if !file.treeReleased {
				file.tree = tree
				file.treeErr = parseErr
			}
			file.treeMu.Unlock()
		},
	)
	file.treeMu.RLock()
	defer file.treeMu.RUnlock()
	if file.treeReleased {
		return nil, fmt.Errorf("parse workspace file %s: CST cache released", file.path)
	}
	return file.tree, file.treeErr
}

// Identity returns the SHA-256 content identity of the immutable source
// snapshot. Calling it on a one-shot workspace loads the source if needed.
func (file *File) Identity() (ContentIdentity, error) {
	if file == nil {
		return ContentIdentity{}, fmt.Errorf("identify workspace file: nil file")
	}
	if file.snapshot != nil {
		return file.snapshot.identity, nil
	}
	contents, err := file.Bytes()
	if err != nil {
		return ContentIdentity{}, err
	}
	return sha256.Sum256(contents), nil
}

// ReleaseCST drops the heavyweight concrete syntax tree after all consumers of
// this file have completed. Source bytes remain cached. A released CST cannot
// be requested again from this immutable workspace generation.
func (file *File) ReleaseCST() {
	if file == nil {
		return
	}
	file.treeMu.Lock()
	file.tree = nil
	file.treeReleased = true
	file.treeMu.Unlock()
}

// Release drops all heavyweight per-file caches after a completed batch job.
// Any byte slices or trees already returned to callers remain valid, but this
// immutable File cannot load them again.
func (file *File) Release() {
	if file == nil {
		return
	}
	file.ReleaseCST()
	file.bytesMu.Lock()
	file.bytes = nil
	file.bytesReleased = true
	file.bytesMu.Unlock()
}
