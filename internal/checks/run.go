package checks

import (
	"fmt"
	"go/token"
	"runtime"
	"sort"
	"sync"

	"github.com/gempir/strider/internal/checks/semantic"
	"github.com/gempir/strider/internal/checks/syntax"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/pathfilter"
	"github.com/gempir/strider/internal/source"
	"github.com/gempir/strider/internal/workspace"
)

// RunOptions configures one read-only check pass.
type RunOptions struct {
	Formatter formatter.Options
	Root      string
	Excludes  []string
	// CollectCandidates retains complete formatted files for a future write or
	// fix operation. Read-only check callers should leave it disabled.
	CollectCandidates bool
}

// Result contains the merged diagnostics and, when requested, formatted
// candidates. Candidates stay internal to the check pipeline so a future fix
// mode can apply them without embedding whole files in diagnostic reports.
type Result struct {
	Diagnostics []diagnostic.Diagnostic
	Candidates  map[string]formatter.Result
}

type fileResult struct {
	filename    string
	diagnostics []diagnostic.Diagnostic
	candidate   *formatter.Result
	err         error
}

type analysisRunner func([]string, *semantic.Registry) ([]diagnostic.Diagnostic, error)

// Run executes the selected checks. Concrete-syntax checks and formatting
// share each workspace file's CST; package-aware checks retain the original
// input patterns so go/packages semantics remain unchanged.
func Run(shared *workspace.Workspace, registry *Registry, options RunOptions) (Result, error) {
	if shared == nil {
		return Result{}, fmt.Errorf("check workspace is nil")
	}
	if registry == nil {
		return Result{}, fmt.Errorf("check registry is nil")
	}
	if options.Formatter.PrintWidth == 0 {
		options.Formatter = formatter.DefaultOptions()
	}

	result, err := runConcreteChecks(shared.Files(), registry, options.Formatter, options.CollectCandidates)
	if err != nil {
		return Result{}, err
	}
	if err := appendAnalysis(&result, shared, registry, semantic.Run); err != nil {
		return Result{}, err
	}
	filterExcludedResults(&result, options.Root, options.Excludes)
	sortDiagnostics(result.Diagnostics)
	if result.Diagnostics == nil {
		result.Diagnostics = []diagnostic.Diagnostic{}
	}
	return result, nil
}

func appendAnalysis(result *Result, shared *workspace.Workspace, registry *Registry, run analysisRunner) error {
	if registry.semantic == nil || len(registry.semantic.Checks()) == 0 {
		return nil
	}
	packageDiagnostics, err := run(shared.Inputs(), registry.semantic)
	if err != nil {
		return err
	}
	for _, item := range packageDiagnostics {
		result.Diagnostics = append(result.Diagnostics, item)
	}
	return nil
}

func filterExcludedResults(result *Result, root string, excludes []string) {
	if len(excludes) == 0 {
		return
	}
	diagnostics := result.Diagnostics[:0]
	for _, item := range result.Diagnostics {
		if !pathfilter.Excluded(root, item.File, excludes) {
			diagnostics = append(diagnostics, item)
		}
	}
	result.Diagnostics = diagnostics
	for filename := range result.Candidates {
		if pathfilter.Excluded(root, filename, excludes) {
			delete(result.Candidates, filename)
		}
	}
}

func runConcreteChecks(files []*workspace.File, registry *Registry, formatOptions formatter.Options, collectCandidates bool) (Result, error) {
	applicable := make([]*workspace.File, 0, len(files))
	for _, file := range files {
		if registry.needsCST(file.Path()) {
			applicable = append(applicable, file)
		}
	}
	result := Result{
		Diagnostics: []diagnostic.Diagnostic{},
	}
	if len(applicable) == 0 {
		return result, nil
	}

	session := formatter.NewFormatter()
	workers := min(runtime.GOMAXPROCS(0), len(applicable))
	jobs := make(chan *workspace.File)
	results := make(chan fileResult, len(applicable))
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for file := range jobs {
				results <- runConcreteFile(file, registry, session, formatOptions, collectCandidates)
			}
		}()
	}
	go func() {
		for _, file := range applicable {
			jobs <- file
		}
		close(jobs)
		group.Wait()
		close(results)
	}()

	completed := make([]fileResult, 0, len(applicable))
	for item := range results {
		completed = append(completed, item)
	}
	sort.Slice(completed, func(left, right int) bool {
		return completed[left].filename < completed[right].filename
	})
	for _, item := range completed {
		if item.err != nil {
			return Result{}, fmt.Errorf("%s: %w", source.DisplayPath(item.filename), item.err)
		}
	}
	for _, item := range completed {
		result.Diagnostics = append(result.Diagnostics, item.diagnostics...)
		if item.candidate != nil {
			if result.Candidates == nil {
				result.Candidates = make(map[string]formatter.Result)
			}
			result.Candidates[item.filename] = *item.candidate
		}
	}
	return result, nil
}

func runConcreteFile(file *workspace.File, registry *Registry, session *formatter.Formatter, formatOptions formatter.Options, collectCandidate bool) fileResult {
	filename := file.Path()
	defer file.Release()
	lintApplies := registry.syntax != nil && registry.syntax.Applies(filename)
	if registry.formatApplies(filename) && !lintApplies {
		contents, readErr := file.Bytes()
		if readErr != nil {
			return fileResult{
				filename: filename,
				err:      readErr,
			}
		}
		if formatter.IsIgnored(contents) {
			return fileResult{
				filename: filename,
			}
		}
	}
	tree, err := file.CST()
	if err != nil {
		return fileResult{
			filename: filename,
			err:      err,
		}
	}
	result := fileResult{
		filename: filename,
	}
	if registry.formatApplies(filename) {
		var formatted formatter.Result
		var formatErr error
		if collectCandidate {
			formatted, formatErr = session.FormatTree(filename, tree, formatOptions)
		} else {
			formatted, formatErr = session.FormatTreeUnverified(filename, tree, formatOptions)
		}
		if formatErr != nil {
			result.err = formatErr
			return result
		}
		if formatted.Changed && !formatted.Ignored {
			result.diagnostics = append(result.diagnostics, formatDiagnostic(filename, registry.Severity(formatMeta.Code)))
			if collectCandidate {
				result.candidate = &formatted
			}
		}
	}
	if lintApplies {
		result.diagnostics = append(result.diagnostics, syntax.AnalyzeTree(filename, tree, registry.syntax)...)
	}
	return result
}

func formatDiagnostic(filename string, severity diagnostic.Severity) diagnostic.Diagnostic {
	display := source.DisplayPath(filename)
	position := token.Position{
		Filename: display,
		Offset:   0,
		Line:     1,
		Column:   1,
	}
	return diagnostic.Diagnostic{
		Code:     formatMeta.Code,
		Message:  "file is not formatted",
		Severity: severity,
		File:     display,
		Start:    position,
		End:      position,
		Fixes: []diagnostic.Fix{
			{
				Message:   fmt.Sprintf("run `strider fmt %s`", display),
				Safety:    diagnostic.Safe,
				Automatic: true,
			},
		},
	}
}

func sortDiagnostics(diagnostics []diagnostic.Diagnostic) {
	sort.SliceStable(
		diagnostics,
		func(leftIndex, rightIndex int) bool {
			left := diagnostics[leftIndex]
			right := diagnostics[rightIndex]
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
