package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gempir/strider/internal/checks"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/fix"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/ui"
	"github.com/gempir/strider/internal/workspace"
)

const checkWatchInterval = time.Second

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

	iteration uint64
}

func runCheck(args []string, configuration config.Config, colorMode ui.ColorMode, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	aliases := map[string]string{
		"format":                           "f",
		"minimum-severity":                 "s",
		"summary-only":                     "q",
		"watch":                            "w",
		"list-checks":                      "l",
		"list-rules":                       "l",
		"explain":                          "e",
		"baseline":                         "b",
		"generate-baseline":                "g",
		"remove-outdated-baseline-entries": "r",
		"only":                             "o",
		"fix":                              "x",
		"fix-unsafe":                       "u",
		"help":                             "h",
	}
	reportFormat := stringOption(flags, "format", "f", "text", "report format: text, json, or html")
	minimumSeverityFlag := stringOption(flags, "minimum-severity", "s", "", "minimum effective severity: none, note, warning, or error")
	summaryOnly := boolOption(flags, "summary-only", "q", false, "only print per-check counts and the aggregate issue summary")
	watch := boolOption(flags, "watch", "w", false, "rerun checks when source changes")
	listChecks := boolOption(flags, "list-checks", "l", false, "list checks admitted by the severity floor")
	flags.BoolVar(listChecks, "list-rules", false, "alias for --list-checks")
	explain := stringOption(flags, "explain", "e", "", "explain a check")
	baselinePath := stringOption(flags, "baseline", "b", "", "path to the check baseline")
	generateBaseline := boolOption(flags, "generate-baseline", "g", false, "replace the baseline with all current findings")
	removeOutdated := boolOption(flags, "remove-outdated-baseline-entries", "r", false, "remove baseline entries that no longer match")
	fixSafe := boolOption(flags, "fix", "x", false, "apply safe automatic fixes")
	fixUnsafe := boolOption(flags, "fix-unsafe", "u", false, "apply all automatic fixes, including unsafe fixes")
	var only stringList
	varOption(flags, &only, "only", "o", "run only these check codes (repeatable or comma-separated)")
	flags.Usage = func() {
		palette := ui.NewPalette(stderr, colorMode)
		fmt.Fprintln(stderr, palette.Accent("Usage:")+" strider check [OPTIONS] [FILE|DIR]...")
		printFlagDefaults(stderr, flags, aliases, palette)
	}
	if !parseCommandFlags(flags, args, aliases, "check", colorMode, stderr) {
		return exitError
	}
	if *reportFormat != "text" && *reportFormat != "json" && *reportFormat != "html" {
		printCommandError(stderr, colorMode, "strider check", "unsupported report format %q", *reportFormat)
		return exitError
	}
	if *watch && *reportFormat != "text" {
		printCommandError(stderr, colorMode, "strider check", "--watch requires text report format")
		return exitError
	}
	if *summaryOnly && *reportFormat != "text" {
		printCommandError(stderr, colorMode, "strider check", "--summary-only requires text report format")
		return exitError
	}
	if *fixSafe && *fixUnsafe {
		printCommandError(stderr, colorMode, "strider check", "--fix and --fix-unsafe are mutually exclusive")
		return exitError
	}
	fixMode := *fixSafe || *fixUnsafe
	if fixMode && *watch {
		printCommandError(stderr, colorMode, "strider check", "fix mode cannot be combined with --watch")
		return exitError
	}
	if fixMode && (*generateBaseline || *removeOutdated) {
		printCommandError(stderr, colorMode, "strider check", "fix mode cannot update a baseline")
		return exitError
	}
	if *watch && (*generateBaseline || *removeOutdated) {
		printCommandError(stderr, colorMode, "strider check", "watch mode cannot update a baseline")
		return exitError
	}

	checkConfig := configuration.Checks
	minimumSeverity, ok := resolveMinimumSeverity(flags, *minimumSeverityFlag, checkConfig.MinimumSeverity, "check", colorMode, stderr)
	if !ok {
		return exitError
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
		printCommandError(stderr, colorMode, "strider check", "%v", err)
		return exitError
	}
	if *listChecks {
		return listChecksInRegistry(registry, colorMode, stdout)
	}
	if *explain != "" {
		return explainCheck(registry, *explain, colorMode, stdout, stderr)
	}

	baselineConfig, ok := resolveBaselineOptions(flags, configuration, checkConfig, *baselinePath, *generateBaseline, *removeOutdated, stderr, "check", colorMode)
	if !ok {
		return exitError
	}
	baselineConfig.selectedCodes = make(map[string]bool, len(registry.Checks()))
	for _, check := range registry.Checks() {
		baselineConfig.selectedCodes[check.Meta().Code] = true
	}
	baselineConfig.knownCodes = registry.KnownCodes()
	workspaceOptions := workspace.Options{
		SkipGenerated: true,
		Root:          configuration.Root,
		Excludes:      checkConfig.Excludes,
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
		if err := runCheckWatch(flags.Args(), workspaceOptions, registry, runOptions, baselineConfig, *summaryOnly, colorMode, stdout, stderr); err != nil {
			printCommandError(stderr, colorMode, "strider check", "%v", err)
			return exitError
		}
		return exitSuccess
	}
	shared, err := workspace.Open(flags.Args(), workspaceOptions)
	if err != nil {
		printCommandError(stderr, colorMode, "strider check", "%v", err)
		return exitError
	}
	var snapshot fix.Snapshot
	if fixMode {
		snapshot, err = fix.Capture(shared)
		if err != nil {
			printCommandError(stderr, colorMode, "strider check", "%v", err)
			return exitError
		}
	}
	result, err := checks.Run(shared, registry, runOptions)
	if err != nil {
		printCommandError(stderr, colorMode, "strider check", "%v", err)
		return exitError
	}
	baselineWriter := stderr
	if fixMode {
		baselineWriter = io.Discard
	}
	diagnostics, handled, err := prepareCheckDiagnostics(result.Diagnostics, baselineConfig, colorMode, baselineWriter)
	if err != nil {
		printCommandError(stderr, colorMode, "strider check", "%v", err)
		return exitError
	}
	if handled {
		return exitSuccess
	}
	if fixMode {
		mode := fix.SafeOnly
		if *fixUnsafe {
			mode = fix.IncludeUnsafe
		}
		formatCheck := configuration.EffectiveCheck("format")
		formatExcludes := append(append([]string(nil), configuration.Formatter.Excludes...), formatCheck.Excludes...)
		fixed, fixErr := fix.Plan(
			snapshot,
			diagnostics,
			result.Candidates,
			fix.Options{
				Mode:           mode,
				Formatter:      runOptions.Formatter,
				Root:           configuration.Root,
				FormatExcludes: formatExcludes,
			},
		)
		if fixErr != nil {
			printCommandError(stderr, colorMode, "strider check", "%v", fixErr)
			return exitError
		}
		palette := ui.NewPalette(stderr, colorMode)
		for _, skipped := range fixed.Skipped {
			fmt.Fprintf(
				stderr,
				"%s skipped %s in %s: %s\n",
				palette.Warning("strider check:"),
				palette.Code(skipped.Code),
				palette.Path(skipped.File),
				skipped.Reason,
			)
		}
		if fixErr = fix.Apply(fixed); fixErr != nil {
			printCommandError(stderr, colorMode, "strider check", "%v", fixErr)
			return exitError
		}

		shared, err = workspace.Open(flags.Args(), workspaceOptions)
		if err != nil {
			printCommandError(stderr, colorMode, "strider check", "%v", err)
			return exitError
		}
		runOptions.CollectCandidates = false
		result, err = checks.Run(shared, registry, runOptions)
		if err != nil {
			printCommandError(stderr, colorMode, "strider check", "%v", err)
			return exitError
		}
		diagnostics, handled, err = prepareCheckDiagnostics(result.Diagnostics, baselineConfig, colorMode, stderr)
		if err != nil {
			printCommandError(stderr, colorMode, "strider check", "%v", err)
			return exitError
		}
		if handled {
			return exitSuccess
		}
	}
	err = reportCheckDiagnostics(stdout, diagnostics, *reportFormat, *summaryOnly, colorMode)
	if err != nil {
		printCommandError(stderr, colorMode, "strider check", "%v", err)
		return exitError
	}
	if len(diagnostics) != 0 {
		return exitFindings
	}
	return exitSuccess
}

func runCheckWatch(
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
	session, err := checks.NewSession(registry, runOptions, checks.SessionOptions{})
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ticker := time.NewTicker(checkWatchInterval)
	defer ticker.Stop()
	lastError := ""
	for {
		if err := watcher.run(); err != nil {
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

func (watcher *checkWatcher) run() error {
	shared, err := watcher.cache.Open(watcher.paths, watcher.workspaceOptions)
	if err != nil {
		return err
	}
	before := watcher.session.Stats()
	result, err := watcher.session.Run(shared)
	if err != nil {
		return err
	}
	after := watcher.session.Stats()
	concreteChanged := after.ConcreteMisses > before.ConcreteMisses
	packageChanged := after.Analysis.Generation > before.Analysis.Generation
	if watcher.iteration != 0 && !concreteChanged && !packageChanged {
		return nil
	}
	diagnostics, handled, err := prepareCheckDiagnostics(result.Diagnostics, watcher.baseline, watcher.colorMode, watcher.stderr)
	if err != nil {
		return err
	}
	if handled {
		return fmt.Errorf("watch mode cannot update a baseline")
	}
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
		return checks.ReportJSON(stdout, diagnostics)
	}
	if reportFormat == "html" {
		return checks.ReportHTML(stdout, diagnostics)
	}
	if summaryOnly {
		return checks.ReportSummary(stdout, diagnostics, colorMode)
	}
	return checks.ReportText(stdout, diagnostics, colorMode)
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
			colorSeverityText(registry.Severity(meta.Code), meta.Code, palette),
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
