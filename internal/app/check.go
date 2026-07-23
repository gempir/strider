package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/gempir/strider/internal/checks"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/fix"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/report"
	"github.com/gempir/strider/internal/ui"
	"github.com/gempir/strider/internal/workspace"
)

const checkWatchInterval = time.Second

var checkConflicts = []checkConflict{
	{
		left:    "watch",
		right:   "structured-report",
		message: "--watch requires text report format",
	},
	{
		left:    "summary-only",
		right:   "structured-report",
		message: "--summary-only requires text report format",
	},
	{
		left:    "fix",
		right:   "fix-unsafe",
		message: "--fix and --fix-unsafe are mutually exclusive",
	},
	{
		left:    "fix-mode",
		right:   "watch",
		message: "fix mode cannot be combined with --watch",
	},
	{
		left:    "fix-mode",
		right:   "baseline-update",
		message: "fix mode cannot update a baseline",
	},
	{
		left:    "watch",
		right:   "baseline-update",
		message: "watch mode cannot update a baseline",
	},
}

type checkWatcher struct {
	paths            []string
	workspaceOptions workspace.Options
	cache            *workspace.Cache
	session          *checks.Session
	baseline         baselineOptions
	summaryOnly      bool
	colorMode        ui.ColorMode
	stdout           io.Writer
	stderr           io.Writer

	iteration       uint64
	hasDiagnostics  bool
	lastDiagnostics []diagnostic.Diagnostic
}

type checkExecution struct {
	paths            []string
	workspaceOptions workspace.Options
	registry         *checks.Registry
	runOptions       checks.RunOptions
	baseline         baselineOptions
	reportFormat     string
	summaryOnly      bool
	fix              bool
	fixMode          fix.Mode
	configuration    config.Config
	colorMode        ui.ColorMode
	stdout           io.Writer
	stderr           io.Writer
}

type checkConflict struct {
	left    string
	right   string
	message string
}

func runCheck(ctx context.Context, args []string, configuration config.Config, colorMode ui.ColorMode, stdout, stderr io.Writer) int {
	exitCode, err := runCheckCommand(ctx, args, configuration, colorMode, stdout, stderr)
	if err != nil {
		printCommandError(stderr, colorMode, "strider check", "%v", err)
		return exitError
	}
	return exitCode
}

func runCheckCommand(ctx context.Context, args []string, configuration config.Config, colorMode ui.ColorMode, stdout, stderr io.Writer) (int, error) {
	if err := ctx.Err(); err != nil {
		return exitError, err
	}
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	aliases := commandOptionAliases["check"]
	reportFormat := stringOption(flags, "format", "f", "text", "report format: text, json, or html")
	minimumSeverityFlag := stringOption(flags, "minimum-severity", "s", "", "minimum effective severity: none, note, warning, or error")
	summaryOnly := boolOption(flags, "summary-only", "q", "only print per-check counts and the aggregate issue summary")
	watch := boolOption(flags, "watch", "w", "rerun checks when source changes")
	listChecks := boolOption(flags, "list-checks", "l", "list checks admitted by the severity floor")
	flags.BoolVar(listChecks, "list-rules", false, "alias for --list-checks")
	explain := stringOption(flags, "explain", "e", "", "explain a check")
	baselinePath := stringOption(flags, "baseline", "b", "", "path to the check baseline")
	generateBaseline := boolOption(flags, "generate-baseline", "g", "replace the baseline with all current findings")
	removeOutdated := boolOption(flags, "remove-outdated-baseline-entries", "r", "remove baseline entries that no longer match")
	fixSafe := boolOption(flags, "fix", "x", "apply safe automatic fixes")
	fixUnsafe := boolOption(flags, "fix-unsafe", "u", "apply all automatic fixes, including unsafe fixes")
	var only stringList
	varOption(flags, &only, "only", "o", "run only these check codes (repeatable or comma-separated)")
	flags.Usage = func() {
		palette := ui.NewPalette(stderr, colorMode)
		fmt.Fprintln(stderr, palette.Accent("Usage:")+" strider check [OPTIONS] [FILE|DIR]...")
		printFlagDefaults(stderr, flags, aliases, palette)
	}
	if !parseCommandFlags(flags, args, aliases, "check", colorMode, stderr) {
		return exitError, nil
	}
	if *reportFormat != "text" && *reportFormat != "json" && *reportFormat != "html" {
		return exitError, fmt.Errorf("unsupported report format %q", *reportFormat)
	}
	fixMode := *fixSafe || *fixUnsafe
	activeModes := map[string]bool{
		"watch":             *watch,
		"structured-report": *reportFormat != "text",
		"summary-only":      *summaryOnly,
		"fix":               *fixSafe,
		"fix-unsafe":        *fixUnsafe,
		"fix-mode":          fixMode,
		"baseline-update":   *generateBaseline || *removeOutdated,
	}
	if err := validateCheckConflicts(activeModes); err != nil {
		return exitError, err
	}

	checkConfig := configuration.Checks
	minimumSeverity, ok := resolveMinimumSeverity(flags, *minimumSeverityFlag, checkConfig.MinimumSeverity, "check", colorMode, stderr)
	if !ok {
		return exitError, nil
	}
	selected := []string(only)
	if *explain != "" {
		selected = []string{
			*explain,
		}
		minimumSeverity = diagnostic.SeverityNone
	}
	registry, err := checks.NewRegistry(
		checks.RegistryOptions{
			Only:            selected,
			Settings:        checkConfig.Settings,
			MinimumSeverity: minimumSeverity,
			FormatExcludes:  configuration.Formatter.Excludes,
			Root:            configuration.Root,
		},
	)
	if err != nil {
		return exitError, err
	}
	if *listChecks {
		return listChecksInRegistry(registry, colorMode, stdout), nil
	}
	if *explain != "" {
		return explainCheck(registry, *explain, colorMode, stdout, stderr), nil
	}

	baselineConfig, ok := resolveBaselineOptions(flags, configuration, checkConfig, *baselinePath, *generateBaseline, *removeOutdated, stderr, "check", colorMode)
	if !ok {
		return exitError, nil
	}
	baselineConfig.selectedCodes = make(map[string]bool, len(registry.Checks()))
	for _, check := range registry.Checks() {
		baselineConfig.selectedCodes[check.Meta().Code] = true
	}
	baselineConfig.knownCodes = registry.KnownCodes()
	workspaceOptions := workspace.Options{
		SkipGenerated: true,
	}
	runOptions := checks.RunOptions{
		Formatter: formatter.Options{
			PrintWidth: configuration.Formatter.PrintWidth,
		},
		Root:              configuration.Root,
		Excludes:          checkConfig.Excludes,
		CollectCandidates: fixMode,
	}
	if *watch {
		if err := runCheckWatch(ctx, flags.Args(), workspaceOptions, registry, runOptions, baselineConfig, *summaryOnly, colorMode, stdout, stderr); err != nil {
			return exitError, err
		}
		return exitSuccess, nil
	}
	mode := fix.SafeOnly
	if *fixUnsafe {
		mode = fix.IncludeUnsafe
	}
	return runCheckOnce(
		ctx,
		checkExecution{
			paths:            flags.Args(),
			workspaceOptions: workspaceOptions,
			registry:         registry,
			runOptions:       runOptions,
			baseline:         baselineConfig,
			reportFormat:     *reportFormat,
			summaryOnly:      *summaryOnly,
			fix:              fixMode,
			fixMode:          mode,
			configuration:    configuration,
			colorMode:        colorMode,
			stdout:           stdout,
			stderr:           stderr,
		},
	)
}

func validateCheckConflicts(active map[string]bool) error {
	for _, conflict := range checkConflicts {
		if active[conflict.left] && active[conflict.right] {
			return fmt.Errorf("%s", conflict.message)
		}
	}
	return nil
}

func runCheckOnce(ctx context.Context, execution checkExecution) (int, error) {
	if err := ctx.Err(); err != nil {
		return exitError, err
	}
	shared, err := workspace.Open(execution.paths, execution.workspaceOptions)
	if err != nil {
		return exitError, err
	}
	if err := ctx.Err(); err != nil {
		return exitError, err
	}
	var snapshot fix.Snapshot
	if execution.fix {
		snapshot, err = fix.Capture(shared, execution.configuration.Root)
		if err != nil {
			return exitError, err
		}
	}
	result, err := checks.Run(ctx, shared, execution.registry, execution.runOptions)
	if err != nil {
		return exitError, err
	}
	baselineWriter := execution.stderr
	if execution.fix {
		baselineWriter = io.Discard
	}
	diagnostics, handled, err := prepareCheckDiagnostics(result.Diagnostics, execution.baseline, execution.colorMode, baselineWriter)
	if err != nil {
		return exitError, err
	}
	if handled {
		return exitSuccess, nil
	}
	if execution.fix {
		diagnostics, handled, err = applyCheckFixes(ctx, execution, snapshot, diagnostics, result.Candidates)
		if err != nil {
			return exitError, err
		}
		if handled {
			return exitSuccess, nil
		}
	}
	err = reportCheckDiagnostics(execution.stdout, diagnostics, execution.reportFormat, execution.summaryOnly, execution.colorMode)
	if err != nil {
		return exitError, err
	}
	if len(diagnostics) != 0 {
		return exitFindings, nil
	}
	return exitSuccess, nil
}

func applyCheckFixes(ctx context.Context, execution checkExecution, snapshot fix.Snapshot, diagnostics []diagnostic.Diagnostic, candidates map[string]formatter.Result) (
	[]diagnostic.Diagnostic,
	bool,
	error,
) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	formatCheck := execution.configuration.EffectiveCheck("format")
	formatExcludes := append(append([]string(nil), execution.configuration.Formatter.Excludes...), formatCheck.Excludes...)
	fixed, err := fix.Plan(
		snapshot,
		diagnostics,
		candidates,
		fix.Options{
			Mode:           execution.fixMode,
			Formatter:      execution.runOptions.Formatter,
			Root:           execution.configuration.Root,
			FormatExcludes: formatExcludes,
			Validate:       checks.ValidateOverlay,
		},
	)
	if err != nil {
		return nil, false, err
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	palette := ui.NewPalette(execution.stderr, execution.colorMode)
	for _, skipped := range fixed.Skipped {
		fmt.Fprintf(
			execution.stderr,
			"%s skipped %s in %s: %s\n",
			palette.Warning("strider check:"),
			palette.Code(skipped.Code),
			palette.Path(skipped.File),
			skipped.Reason,
		)
	}
	if err := fix.Apply(fixed); err != nil {
		return nil, false, err
	}
	shared, err := workspace.Open(execution.paths, execution.workspaceOptions)
	if err != nil {
		return nil, false, err
	}
	runOptions := execution.runOptions
	runOptions.CollectCandidates = false
	result, err := checks.Run(ctx, shared, execution.registry, runOptions)
	if err != nil {
		return nil, false, err
	}
	return prepareCheckDiagnostics(result.Diagnostics, execution.baseline, execution.colorMode, execution.stderr)
}

func runCheckWatch(
	ctx context.Context,
	paths []string,
	workspaceOptions workspace.Options,
	registry *checks.Registry,
	runOptions checks.RunOptions,
	baselineConfig baselineOptions,
	summaryOnly bool,
	colorMode ui.ColorMode,
	stdout,
	stderr io.Writer,
) error {
	session, err := checks.NewSession(registry, runOptions)
	if err != nil {
		return err
	}
	watcher := &checkWatcher{
		paths:            append([]string(nil), paths...),
		workspaceOptions: workspaceOptions,
		cache:            workspace.NewCache(workspace.CacheOptions{}),
		session:          session,
		baseline:         baselineConfig,
		summaryOnly:      summaryOnly,
		colorMode:        colorMode,
		stdout:           stdout,
		stderr:           stderr,
	}
	ticker := time.NewTicker(checkWatchInterval)
	defer ticker.Stop()
	lastError := ""
	for {
		if err := watcher.run(ctx); err != nil {
			if err.Error() != lastError {
				printCommandError(stderr, colorMode, "strider check", "%v", err)
				lastError = err.Error()
			}
		} else {
			lastError = ""
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (watcher *checkWatcher) run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	shared, err := watcher.cache.Open(watcher.paths, watcher.workspaceOptions)
	if err != nil {
		return err
	}
	before := watcher.session.Stats()
	if err := ctx.Err(); err != nil {
		return err
	}
	result, err := watcher.session.Run(ctx, shared)
	if err != nil {
		return err
	}
	after := watcher.session.Stats()
	concreteChanged := after.ConcreteMisses > before.ConcreteMisses
	diagnostics, handled, err := prepareCheckDiagnostics(result.Diagnostics, watcher.baseline, watcher.colorMode, watcher.stderr)
	if err != nil {
		return err
	}
	if handled {
		return fmt.Errorf("watch mode cannot update a baseline")
	}
	diagnosticsChanged := !watcher.hasDiagnostics || !reflect.DeepEqual(watcher.lastDiagnostics, diagnostics)
	if watcher.iteration != 0 && !concreteChanged && !diagnosticsChanged {
		return nil
	}
	watcher.lastDiagnostics = diagnostics
	watcher.hasDiagnostics = true
	watcher.iteration++
	fmt.Fprintf(watcher.stdout, "== strider check #%d ==\n", watcher.iteration)
	if err := reportCheckDiagnostics(watcher.stdout, diagnostics, "text", watcher.summaryOnly, watcher.colorMode); err != nil {
		return err
	}
	if len(diagnostics) == 0 && !watcher.summaryOnly {
		fmt.Fprintln(watcher.stdout, "No findings.")
	}
	return nil
}

func prepareCheckDiagnostics(diagnostics []diagnostic.Diagnostic, baselineConfig baselineOptions, colorMode ui.ColorMode, stderr io.Writer) ([]diagnostic.Diagnostic, bool, error) {
	if baselineConfig.generate {
		diagnostics = withoutFormatDiagnostics(diagnostics)
	}
	return applyBaseline("check", diagnostics, baselineConfig, colorMode, stderr)
}

func reportCheckDiagnostics(stdout io.Writer, diagnostics []diagnostic.Diagnostic, reportFormat string, summaryOnly bool, colorMode ui.ColorMode) error {
	if reportFormat == "json" {
		return report.JSON(stdout, diagnostics)
	}
	if reportFormat == "html" {
		return report.HTML(stdout, "Strider check report", diagnostics)
	}
	if summaryOnly {
		return report.TextWithOptions(stdout, diagnostics, colorMode, report.TextOptions{
			SummaryOnly: true,
		})
	}
	return report.Text(stdout, diagnostics, colorMode)
}

func listChecksInRegistry(registry *checks.Registry, colorMode ui.ColorMode, stdout io.Writer) int {
	palette := ui.NewPalette(stdout, colorMode)
	entries := make([]checkListEntry, 0, len(registry.Checks()))
	for _, check := range registry.Checks() {
		meta := check.Meta()
		entries = append(entries, checkListEntry{
			code:     meta.Code,
			severity: registry.Severity(meta.Code),
			summary:  meta.Summary,
		})
	}
	writeCheckList(stdout, palette, entries)
	return exitSuccess
}

func explainCheck(registry *checks.Registry, code string, colorMode ui.ColorMode, stdout, stderr io.Writer) int {
	palette := ui.NewPalette(stdout, colorMode)
	for _, check := range registry.Checks() {
		meta := check.Meta()
		if !strings.EqualFold(meta.Code, code) {
			continue
		}
		fmt.Fprintf(
			stdout,
			"%s (%s)\n\n%s\n\n%s\n%s\n\n%s\n%s\n",
			report.StyleSeverity(registry.Severity(meta.Code), meta.Code, palette),
			colorSeverity(registry.Severity(meta.Code), palette),
			meta.Explanation,
			palette.Success("Good:"),
			meta.GoodExample,
			palette.Error("Bad:"),
			meta.BadExample,
		)
		return exitSuccess
	}
	printCommandError(stderr, colorMode, "strider check", "unknown check %q", code)
	return exitError
}

func withoutFormatDiagnostics(items []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	filtered := make([]diagnostic.Diagnostic, 0, len(items))
	for _, item := range items {
		if item.Code != "format" {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
