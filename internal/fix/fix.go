// Package fix selects, composes, validates, and applies automatic source fixes.
package fix

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"

	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/filewrite"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/pathfilter"
	"github.com/gempir/strider/internal/source"
	"github.com/gempir/strider/internal/workspace"
)

const (
	// SafeOnly applies only fixes explicitly classified as safe.
	SafeOnly Mode = iota
	// IncludeUnsafe applies safe, potentially unsafe, and unsafe fixes.
	IncludeUnsafe
)

// Mode controls which automatic fixes are eligible.
type Mode uint8

// FileSnapshot is the immutable source input used by the check pass.
type FileSnapshot struct {
	Path         string
	ResolvedPath string
	Before       []byte
	Identity     workspace.ContentIdentity
}

// Snapshot captures every discovered source before checks release their
// workspace caches. The private diagnostic-path index admits only the single
// root-relative spelling produced by source.DiagnosticPath.
type Snapshot struct {
	Inputs                []string
	Files                 map[string]FileSnapshot
	filesByDiagnosticPath map[string]string
}

// Options configures fix selection and post-edit formatting.
type Options struct {
	Mode           Mode
	Formatter      formatter.Options
	Root           string
	FormatExcludes []string
	Validate       func(paths []string, sources map[string][]byte) error
}

// Change is one fully composed and validated file replacement.
type Change struct {
	Path   string
	Before []byte
	After  []byte
}

// Skipped describes an automatic fix that could not be composed safely.
type Skipped struct {
	File   string
	Code   string
	Reason string
}

// Result is a staged fix plan. Apply performs the final stale checks and
// filesystem writes.
type Result struct {
	Changes []Change
	Applied int
	Skipped []Skipped
	guards  []filewrite.Guard
}

type proposal struct {
	code   string
	file   FileSnapshot
	safety diagnostic.Safety
	edits  []diagnostic.TextEdit
	format bool
}

// Capture retains the exact source generation that checks will analyze.
func Capture(shared *workspace.Workspace, root string) (Snapshot, error) {
	if shared == nil {
		return Snapshot{}, fmt.Errorf("capture fixes: nil workspace")
	}
	snapshot := Snapshot{
		Inputs:                shared.Inputs(),
		Files:                 make(map[string]FileSnapshot),
		filesByDiagnosticPath: make(map[string]string),
	}
	targets := make(map[string]string)
	type capturedTarget struct {
		path     string
		resolved filewrite.ResolvedFile
	}
	identities := make(map[workspace.ContentIdentity][]capturedTarget)
	for _, file := range shared.Files() {
		resolved, err := filewrite.ResolveExisting(file.Path())
		if err != nil {
			return Snapshot{}, fmt.Errorf("capture fixes for %s: %w", source.DisplayPath(file.Path()), err)
		}
		path := resolved.Path
		contents, err := file.Bytes()
		if err != nil {
			return Snapshot{}, fmt.Errorf("capture fixes for %s: %w", source.DisplayPath(path), err)
		}
		identity, err := file.Identity()
		if err != nil {
			return Snapshot{}, fmt.Errorf("capture fixes for %s: %w", source.DisplayPath(path), err)
		}
		if previous, exists := targets[resolved.Target]; exists && previous != path {
			return Snapshot{}, fmt.Errorf("capture fixes: %s and %s resolve to the same source", source.DisplayPath(previous), source.DisplayPath(path))
		}
		for _, previous := range identities[identity] {
			if previous.resolved.Target != resolved.Target && previous.resolved.Same(resolved) {
				return Snapshot{}, fmt.Errorf("capture fixes: %s and %s identify the same source", source.DisplayPath(previous.path), source.DisplayPath(path))
			}
		}
		targets[resolved.Target] = path
		identities[identity] = append(identities[identity], capturedTarget{
			path:     path,
			resolved: resolved,
		})
		snapshot.Files[path] = FileSnapshot{
			Path:         path,
			ResolvedPath: resolved.Target,
			Before:       append([]byte(nil), contents...),
			Identity:     identity,
		}
		diagnosticPath := source.DiagnosticPath(root, path)
		if previous, exists := snapshot.filesByDiagnosticPath[diagnosticPath]; exists && previous != path {
			return Snapshot{}, fmt.Errorf("capture fixes: ambiguous diagnostic path %q", diagnosticPath)
		}
		snapshot.filesByDiagnosticPath[diagnosticPath] = path
	}
	return snapshot, nil
}

func (snapshot Snapshot) resolve(path string) (FileSnapshot, bool) {
	resolved, ok := snapshot.filesByDiagnosticPath[path]
	if !ok {
		return FileSnapshot{}, false
	}
	file, ok := snapshot.Files[resolved]
	return file, ok
}

// Plan selects automatic fixes from visible diagnostics, composes every file
// in memory, formats granular edits, and validates the complete overlay.
func Plan(snapshot Snapshot, diagnostics []diagnostic.Diagnostic, candidates map[string]formatter.Result, options Options) (Result, error) {
	if options.Mode != SafeOnly && options.Mode != IncludeUnsafe {
		return Result{}, fmt.Errorf("plan fixes: invalid mode %d", options.Mode)
	}
	if options.Formatter.PrintWidth == 0 {
		options.Formatter = formatter.DefaultOptions()
	}
	proposals := make([]proposal, 0, len(diagnostics))
	for _, item := range diagnostics {
		automatic, found, err := automaticFix(item)
		if err != nil {
			return Result{}, err
		}
		if !found || !allowed(automatic.Safety, options.Mode) {
			continue
		}
		file, ok := snapshot.resolve(item.File)
		if !ok {
			return Result{}, fmt.Errorf("plan fix %s for %s: diagnostic file was not in the analyzed workspace", item.Code, item.File)
		}
		current := proposal{
			code:   item.Code,
			file:   file,
			safety: automatic.Safety,
		}
		if item.Code == "format" {
			if len(automatic.Edits) != 0 {
				return Result{}, fmt.Errorf("plan fix format for %s: formatter fix must not expose text edits", item.File)
			}
			if _, ok := formatterCandidate(snapshot, candidates, file.Path); !ok {
				return Result{}, fmt.Errorf("plan fix format for %s: validated formatter candidate is missing", item.File)
			}
			current.format = true
		} else {
			edits, err := normalizeEdits(item, automatic, file.Before)
			if err != nil {
				return Result{}, err
			}
			if len(edits) == 0 {
				return Result{}, fmt.Errorf("plan fix %s for %s: automatic fix makes no changes", item.Code, item.File)
			}
			current.edits = edits
		}
		proposals = append(proposals, current)
	}

	conflicts := conflictingProposals(proposals)
	result := Result{}
	acceptedByFile := make(map[string][]proposal)
	formatFiles := make(map[string]bool)
	validateTypes := false
	for index, current := range proposals {
		if conflicts[index] {
			result.Skipped = append(
				result.Skipped,
				Skipped{
					File:   source.DisplayPath(current.file.Path),
					Code:   current.code,
					Reason: "overlaps another non-identical automatic fix",
				},
			)
			continue
		}
		result.Applied++
		if current.safety == diagnostic.Safe {
			validateTypes = true
		}
		if current.format {
			formatFiles[current.file.Path] = true
			continue
		}
		acceptedByFile[current.file.Path] = append(acceptedByFile[current.file.Path], current)
	}

	paths := make([]string, 0, len(acceptedByFile)+len(formatFiles))
	seenPaths := make(map[string]bool)
	for path := range acceptedByFile {
		paths = append(paths, path)
		seenPaths[path] = true
	}
	for path := range formatFiles {
		if !seenPaths[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)

	finalSources := make(map[string][]byte, len(snapshot.Files)*2)
	for path, file := range snapshot.Files {
		finalSources[path] = append([]byte(nil), file.Before...)
		finalSources[file.ResolvedPath] = append([]byte(nil), file.Before...)
		result.guards = append(result.guards, filewrite.Guard{
			Path:           path,
			Before:         file.Before,
			ExpectedTarget: file.ResolvedPath,
		})
	}
	sort.Slice(result.guards, func(left, right int) bool {
		return result.guards[left].Path < result.guards[right].Path
	})

	for _, path := range paths {
		file := snapshot.Files[path]
		after := append([]byte(nil), file.Before...)
		if selected := acceptedByFile[path]; len(selected) != 0 {
			after = applyEdits(after, selected)
			if !formatExcluded(options.Root, file.Path, options.FormatExcludes) {
				formatted, err := formatter.FormatWithOptions(path, after, options.Formatter)
				if err != nil {
					return Result{}, fmt.Errorf("format fixes for %s: %w", source.DisplayPath(path), err)
				}
				after = formatted.Source
			}
		} else {
			candidate, _ := formatterCandidate(snapshot, candidates, path)
			after = append([]byte(nil), candidate.Source...)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), path, after, parser.AllErrors); err != nil {
			return Result{}, fmt.Errorf("validate fixes for %s: %w", source.DisplayPath(path), err)
		}
		finalSources[path] = after
		finalSources[file.ResolvedPath] = after
		if !bytes.Equal(file.Before, after) {
			result.Changes = append(result.Changes, Change{
				Path:   path,
				Before: file.Before,
				After:  after,
			})
		}
	}

	if validateTypes && len(result.Changes) != 0 {
		if options.Validate == nil {
			return Result{}, fmt.Errorf("plan fixes: validator is required for safe fixes")
		}
		changedPaths := make([]string, 0, len(result.Changes))
		for _, change := range result.Changes {
			changedPaths = append(changedPaths, snapshot.Files[change.Path].ResolvedPath)
		}
		if err := options.Validate(changedPaths, finalSources); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}

func formatExcluded(root, path string, patterns []string) bool {
	if pathfilter.Excluded(root, path, patterns) {
		return true
	}
	resolvedDirectory, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return false
	}
	logicalPath := filepath.Join(resolvedDirectory, filepath.Base(path))
	return logicalPath != path && pathfilter.Excluded(root, logicalPath, patterns)
}

func automaticFix(item diagnostic.Diagnostic) (diagnostic.Fix, bool, error) {
	var selected diagnostic.Fix
	found := false
	for _, candidate := range item.Fixes {
		if !candidate.Automatic {
			continue
		}
		if found {
			return diagnostic.Fix{}, false, fmt.Errorf("plan fix %s for %s: diagnostic has more than one automatic fix", item.Code, item.File)
		}
		if !diagnostic.ValidSafety(candidate.Safety) {
			return diagnostic.Fix{}, false, fmt.Errorf("plan fix %s for %s: invalid safety %q", item.Code, item.File, candidate.Safety)
		}
		selected = candidate
		found = true
	}
	return selected, found, nil
}

func allowed(safety diagnostic.Safety, mode Mode) bool {
	return safety == diagnostic.Safe || mode == IncludeUnsafe
}

func normalizeEdits(item diagnostic.Diagnostic, fix diagnostic.Fix, before []byte) ([]diagnostic.TextEdit, error) {
	edits := append([]diagnostic.TextEdit(nil), fix.Edits...)
	sort.Slice(
		edits,
		func(left, right int) bool {
			if edits[left].Start != edits[right].Start {
				return edits[left].Start < edits[right].Start
			}
			if edits[left].End != edits[right].End {
				return edits[left].End < edits[right].End
			}
			return edits[left].NewText < edits[right].NewText
		},
	)
	result := edits[:0]
	for _, edit := range edits {
		if edit.Start < 0 || edit.End < edit.Start || edit.End > len(before) {
			return nil, fmt.Errorf("plan fix %s for %s: invalid edit range [%d,%d) for %d-byte source", item.Code, item.File, edit.Start, edit.End, len(before))
		}
		if edit.OldText != "" && string(before[edit.Start:edit.End]) != edit.OldText {
			return nil, fmt.Errorf(
				"plan fix %s for %s: edit range [%d,%d) did not contain expected source %q",
				item.Code,
				item.File,
				edit.Start,
				edit.End,
				edit.OldText,
			)
		}
		if bytes.Equal(before[edit.Start:edit.End], []byte(edit.NewText)) {
			continue
		}
		if len(result) != 0 && equalEdit(result[len(result)-1], edit) {
			continue
		}
		if len(result) != 0 && editsOverlap(result[len(result)-1], edit) {
			return nil, fmt.Errorf("plan fix %s for %s: automatic fix contains overlapping edits", item.Code, item.File)
		}
		result = append(result, edit)
	}
	return result, nil
}

func conflictingProposals(proposals []proposal) map[int]bool {
	conflicts := make(map[int]bool)
	for left := range proposals {
		if proposals[left].format {
			continue
		}
		for right := left + 1; right < len(proposals); right++ {
			if proposals[right].format || proposals[left].file.Path != proposals[right].file.Path || equalEditSets(proposals[left].edits, proposals[right].edits) {
				continue
			}
			if editSetsOverlap(proposals[left].edits, proposals[right].edits) {
				conflicts[left] = true
				conflicts[right] = true
			}
		}
	}
	return conflicts
}

func equalEditSets(left, right []diagnostic.TextEdit) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !equalEdit(left[index], right[index]) {
			return false
		}
	}
	return true
}

func editSetsOverlap(left, right []diagnostic.TextEdit) bool {
	for _, leftEdit := range left {
		for _, rightEdit := range right {
			if editsOverlap(leftEdit, rightEdit) {
				return true
			}
		}
	}
	return false
}

func equalEdit(left, right diagnostic.TextEdit) bool {
	return left.Start == right.Start && left.End == right.End && left.NewText == right.NewText
}

func editsOverlap(left, right diagnostic.TextEdit) bool {
	if left.Start == left.End && right.Start == right.End {
		return left.Start == right.Start
	}
	if left.Start == left.End {
		return left.Start >= right.Start && left.Start <= right.End
	}
	if right.Start == right.End {
		return right.Start >= left.Start && right.Start <= left.End
	}
	return left.Start < right.End && right.Start < left.End
}

func applyEdits(before []byte, proposals []proposal) []byte {
	sets := make([][]diagnostic.TextEdit, 0, len(proposals))
	for _, current := range proposals {
		duplicate := false
		for _, existing := range sets {
			if equalEditSets(existing, current.edits) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			sets = append(sets, current.edits)
		}
	}
	edits := make([]diagnostic.TextEdit, 0)
	for _, set := range sets {
		edits = append(edits, set...)
	}
	sort.Slice(
		edits,
		func(left, right int) bool {
			if edits[left].Start != edits[right].Start {
				return edits[left].Start > edits[right].Start
			}
			return edits[left].End > edits[right].End
		},
	)
	result := append([]byte(nil), before...)
	for _, edit := range edits {
		next := make([]byte, 0, len(result)-(edit.End-edit.Start)+len(edit.NewText))
		next = append(next, result[:edit.Start]...)
		next = append(next, edit.NewText...)
		next = append(next, result[edit.End:]...)
		result = next
	}
	return result
}

func formatterCandidate(snapshot Snapshot, candidates map[string]formatter.Result, path string) (formatter.Result, bool) {
	if _, ok := snapshot.Files[path]; !ok {
		return formatter.Result{}, false
	}
	candidate, ok := candidates[path]
	if ok && candidate.Changed && !candidate.Ignored {
		return candidate, true
	}
	return formatter.Result{}, false
}

// Apply performs stale-source checks, stages the complete batch, and writes
// every planned change atomically per file.
func Apply(result Result) error {
	changes := make([]filewrite.Change, 0, len(result.Changes))
	for _, change := range result.Changes {
		changes = append(changes, filewrite.Change{
			Path:   change.Path,
			Before: change.Before,
			After:  change.After,
		})
	}
	if err := filewrite.WriteBatch(changes, result.guards...); err != nil {
		return fmt.Errorf("apply fixes: %w", err)
	}
	return nil
}
