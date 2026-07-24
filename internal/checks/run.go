//strider:ignore-file cognitive-complexity,cyclomatic-complexity,function-length,modifies-parameter,use-errors-new,use-slices-sort
package checks

import (
	"context"
	"errors"
	"fmt"
	"go/token"
	"os"
	"runtime"
	"sort"
	"strconv"

	"github.com/gempir/strider/internal/checks/semantic"
	"github.com/gempir/strider/internal/checks/syntax"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/fileschedule"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/pathfilter"
	"github.com/gempir/strider/internal/resultcache"
	"github.com/gempir/strider/internal/source"
	"github.com/gempir/strider/internal/telemetry"
	"github.com/gempir/strider/internal/workspace"
)

const SemanticOverlapEnvironment = "STRIDER_OVERLAP_PACKAGE_LOADING"

// RunOptions configures one read-only check pass.
type RunOptions struct {
	Formatter formatter.Options
	Root      string
	Excludes  []string
	// SkipPackageLoading omits package-aware checks and avoids invoking the Go
	// package loader. Concrete-syntax checks and formatting still run.
	SkipPackageLoading bool
	// CollectCandidates retains complete formatted files for a future write or
	// fix operation. Read-only check callers should leave it disabled.
	CollectCandidates bool
	// Cache persists file-local format status and native syntax findings for
	// read-only one-shot commands. Candidate-producing runs bypass it.
	Cache *resultcache.Cache
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

type analysisRunner func(context.Context, []string, *semantic.Plan) ([]diagnostic.Diagnostic, error)

type concreteFileRunner func(context.Context, *workspace.File, *Registry, *formatter.Formatter, formatter.Options, bool) fileResult

type semanticResult struct {
	diagnostics []diagnostic.Diagnostic
	err         error
}

// Run executes the selected checks. Concrete-syntax checks and formatting
// share each workspace file's CST; package-aware checks retain the original
// input patterns so go/packages semantics remain unchanged.
func Run(ctx context.Context, shared *workspace.Workspace, registry *Registry, options RunOptions) (Result, error) {
	finish := telemetry.Start("check.total")
	defer finish()
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if shared == nil {
		return Result{}, fmt.Errorf("check workspace is nil")
	}
	if registry == nil {
		return Result{}, fmt.Errorf("check registry is nil")
	}
	if options.Formatter.PrintWidth == 0 {
		options.Formatter = formatter.DefaultOptions()
	}

	var result Result
	var err error
	overlap := !options.SkipPackageLoading && registry.semantic != nil && os.Getenv(SemanticOverlapEnvironment) == "1"
	telemetry.Attribute("semantic_overlap", strconv.FormatBool(overlap))
	if overlap {
		result, err = runOverlappedChecks(ctx, shared, registry, options)
	} else {
		result, err = runConcreteChecks(ctx, shared.Files(), registry, options.Formatter, options.CollectCandidates, options.Cache)
		if err == nil && !options.SkipPackageLoading {
			err = appendAnalysis(ctx, &result, shared, registry, semantic.RunContext)
		}
	}
	if err != nil {
		return Result{}, err
	}
	filterExcludedResults(&result, options.Root, options.Excludes)
	sortFinish := telemetry.Start("check.sort")
	diagnostic.Sort(result.Diagnostics)
	sortFinish()
	if result.Diagnostics == nil {
		result.Diagnostics = []diagnostic.Diagnostic{}
	}
	return result, nil
}

func runOverlappedChecks(ctx context.Context, shared *workspace.Workspace, registry *Registry, options RunOptions) (Result, error) {
	overlapContext, cancel := context.WithCancel(ctx)
	defer cancel()
	semanticResults := make(chan semanticResult, 1)
	go func() {
		diagnostics, err := semantic.RunContext(overlapContext, shared.Inputs(), registry.semantic)
		if err != nil {
			cancel()
		}
		semanticResults <- semanticResult{
			diagnostics: diagnostics,
			err:         err,
		}
	}()
	result, concreteErr := runConcreteChecks(overlapContext, shared.Files(), registry, options.Formatter, options.CollectCandidates, options.Cache)
	if concreteErr != nil {
		cancel()
	}
	analysis := <-semanticResults
	if concreteErr != nil {
		return Result{}, concreteErr
	}
	if analysis.err != nil {
		return Result{}, analysis.err
	}
	result.Diagnostics = append(result.Diagnostics, analysis.diagnostics...)
	return result, nil
}

func appendAnalysis(ctx context.Context, result *Result, shared *workspace.Workspace, registry *Registry, run analysisRunner) error {
	if registry.semantic == nil {
		return nil
	}
	packageDiagnostics, err := run(ctx, shared.Inputs(), registry.semantic)
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

func runConcreteChecks(ctx context.Context, files []*workspace.File, registry *Registry, formatOptions formatter.Options, collectCandidates bool, cache *resultcache.Cache) (
	Result,
	error,
) {
	restoreGC := workspace.BeginCSTCollectionWindow()
	defer restoreGC()
	admission := workspace.NewCSTAdmission(0)
	runFile := func(ctx context.Context, file *workspace.File, registry *Registry, session *formatter.Formatter, formatOptions formatter.Options, collectCandidate bool) fileResult {
		activeCache := cache
		if collectCandidate {
			activeCache = nil
		}
		return runConcreteFileCached(ctx, file, registry, session, formatOptions, collectCandidate, activeCache, admission)
	}
	result, err := runConcreteChecksWith(ctx, files, registry, formatOptions, collectCandidates, runFile)
	telemetry.Snapshot("check.after-file-local")
	return result, err
}

func runConcreteChecksWith(ctx context.Context, files []*workspace.File, registry *Registry, formatOptions formatter.Options, collectCandidates bool, runFile concreteFileRunner) (
	Result,
	error,
) {
	finish := telemetry.Start("check.file-local")
	defer finish()
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
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
	strategy, err := fileschedule.Resolve()
	if err != nil {
		return Result{}, err
	}
	applicable, err = fileschedule.Order(applicable, strategy, func(file *workspace.File) (int64, error) {
		contents, readErr := file.Bytes()
		return int64(len(contents)), readErr
	})
	if err != nil {
		return Result{}, err
	}
	telemetry.Attribute("file_scheduler", string(strategy))

	session := formatter.NewFormatter()
	workerContext, cancel := context.WithCancel(ctx)
	defer cancel()
	workers := min(runtime.GOMAXPROCS(0), len(applicable))
	results := make(chan fileResult, len(applicable))
	fileschedule.Run(
		workerContext,
		strategy,
		workers,
		applicable,
		func(_ context.Context, file *workspace.File) {
			item := runFile(workerContext, file, registry, session, formatOptions, collectCandidates)
			results <- item
			if item.err != nil {
				cancel()
			}
		},
	)
	close(results)

	completed := make([]fileResult, 0, len(applicable))
	for item := range results {
		completed = append(completed, item)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	sort.Slice(completed, func(left, right int) bool {
		return completed[left].filename < completed[right].filename
	})
	for _, item := range completed {
		if item.err != nil && !errors.Is(item.err, context.Canceled) && !errors.Is(item.err, context.DeadlineExceeded) {
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

func runConcreteFile(ctx context.Context, file *workspace.File, registry *Registry, session *formatter.Formatter, formatOptions formatter.Options, collectCandidate bool) fileResult {
	return runConcreteFileCached(ctx, file, registry, session, formatOptions, collectCandidate, nil, nil)
}

func runConcreteFileCached(
	ctx context.Context,
	file *workspace.File,
	registry *Registry,
	session *formatter.Formatter,
	formatOptions formatter.Options,
	collectCandidate bool,
	cache *resultcache.Cache,
	admission *workspace.CSTAdmission,
) fileResult {
	finish := telemetry.Start("check.file-worker")
	defer finish()
	defer file.Release()
	filename := file.Path()
	if err := ctx.Err(); err != nil {
		return fileResult{
			filename: filename,
			err:      err,
		}
	}
	lintApplies := registry.syntax != nil && registry.syntax.Applies(filename)
	formatApplies := registry.formatApplies(filename)
	formatIgnored := false
	contents, readErr := file.Bytes()
	if readErr != nil {
		return fileResult{
			filename: filename,
			err:      readErr,
		}
	}
	if formatApplies {
		formatIgnored = formatter.IsIgnored(contents)
	}
	display := registry.diagnosticPath(filename)
	cacheKey := localCacheKey(cache, session, registry, filename, display, contents, formatOptions)
	if cached, ok := cache.Get(cacheKey); ok {
		return materializeCachedFile(filename, display, contents, registry, cached)
	}
	if formatIgnored && !lintApplies {
		cache.Store(cacheKey, resultcache.Entry{
			FormatKnown:   true,
			FormatIgnored: true,
		})
		return fileResult{
			filename: filename,
		}
	}
	if err := ctx.Err(); err != nil {
		return fileResult{
			filename: filename,
			err:      err,
		}
	}
	releaseAdmission, err := admission.Acquire(ctx, workspace.EstimatedCSTBytes(int64(len(contents))))
	if err != nil {
		return fileResult{
			filename: filename,
			err:      err,
		}
	}
	defer releaseAdmission()
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
	if err := ctx.Err(); err != nil {
		result.err = err
		return result
	}
	if formatApplies && !formatIgnored {
		if collectCandidate {
			formatted, formatErr := session.FormatTreeKnownActive(filename, tree, formatOptions)
			if formatErr != nil {
				result.err = formatErr
				return result
			}
			if err := ctx.Err(); err != nil {
				result.err = err
				return result
			}
			if formatted.Changed && !formatted.Ignored {
				result.diagnostics = append(result.diagnostics, formatDiagnostic(display, registry.Severity(formatMeta.Code)))
				result.candidate = &formatted
			}
		} else {
			changed, formatErr := session.WouldChangeTreeKnownActive(filename, tree, formatOptions)
			if formatErr != nil {
				result.err = formatErr
				return result
			}
			if err := ctx.Err(); err != nil {
				result.err = err
				return result
			}
			if changed {
				result.diagnostics = append(result.diagnostics, formatDiagnostic(display, registry.Severity(formatMeta.Code)))
			}
		}
	}
	if lintApplies {
		syntaxDiagnostics := syntax.AnalyzeTreeWithDisplay(filename, display, tree, registry.syntax)
		result.diagnostics = append(result.diagnostics, syntaxDiagnostics...)
		cache.Store(
			cacheKey,
			resultcache.Entry{
				FormatKnown:   formatApplies,
				FormatChanged: hasDiagnosticCode(result.diagnostics, formatMeta.Code),
				FormatIgnored: formatIgnored,
				Findings:      resultcache.FindingsFromDiagnostics(syntaxDiagnostics),
			},
		)
	} else {
		cache.Store(
			cacheKey,
			resultcache.Entry{
				FormatKnown:   formatApplies,
				FormatChanged: hasDiagnosticCode(result.diagnostics, formatMeta.Code),
				FormatIgnored: formatIgnored,
			},
		)
	}
	return result
}

func localCacheKey(cache *resultcache.Cache, session *formatter.Formatter, registry *Registry, filename, logicalPath string, contents []byte, formatOptions formatter.Options) string {
	if cache == nil {
		return ""
	}
	target := fmt.Sprintf("goos=%s\ngoarch=%s\ncgo=%s\nskip-generated=true", os.Getenv("GOOS"), os.Getenv("GOARCH"), os.Getenv("CGO_ENABLED"))
	return cache.Key(
		[]byte("check-file-local"),
		contents,
		[]byte(logicalPath),
		[]byte(registry.localCacheIdentity(filename)),
		[]byte(session.CacheIdentity(filename, formatOptions)),
		[]byte(target),
	)
}

func materializeCachedFile(filename, display string, contents []byte, registry *Registry, cached resultcache.Entry) fileResult {
	result := fileResult{
		filename: filename,
	}
	if cached.FormatKnown && cached.FormatChanged && !cached.FormatIgnored {
		result.diagnostics = append(result.diagnostics, formatDiagnostic(display, registry.Severity(formatMeta.Code)))
	}
	result.diagnostics = append(result.diagnostics, resultcache.Materialize(display, contents, cached.Findings, registry.Severity)...)
	return result
}

func hasDiagnosticCode(diagnostics []diagnostic.Diagnostic, code string) bool {
	for _, item := range diagnostics {
		if item.Code == code {
			return true
		}
	}
	return false
}

func formatDiagnostic(display string, severity diagnostic.Severity) diagnostic.Diagnostic {
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
