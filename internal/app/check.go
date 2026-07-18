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

	checkengine "github.com/gempir/strider/internal/checks"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/ui"
	"github.com/gempir/strider/internal/workspace"
)

func runCheck(args []string, configuration config.Config, colorMode ui.ColorMode, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	aliases := map[string]string{
		"format": "f",
		"minimum-severity": "s",
		"summary-only": "q",
		"watch": "w",
		"list-checks": "l",
		"list-rules": "l",
		"all": "a",
		"all-rules": "a",
		"explain": "e",
		"baseline": "b",
		"baseline-variant": "v",
		"generate-baseline": "g",
		"remove-outdated-baseline-entries": "r",
		"ignore-baseline": "i",
		"backup-baseline": "B",
		"only": "o",
		"help": "h",
	}
	reportFormat := stringOption(flags, "format", "f", "text", "report format: text, json, or html")
	minimumSeverityFlag := stringOption(flags, "minimum-severity", "s", "", "minimum effective severity: note, warning, or error")
	summaryOnly := boolOption(flags, "summary-only", "q", false, "only print per-check counts and the aggregate issue summary")
	watch := boolOption(flags, "watch", "w", false, "rerun checks when source changes")
	listChecks := boolOption(flags, "list-checks", "l", false, "list enabled checks")
	flags.BoolVar(listChecks, "list-rules", false, "alias for --list-checks")
	allChecks := boolOption(flags, "all", "a", false, "run every built-in check")
	flags.BoolVar(allChecks, "all-rules", false, "alias for --all")
	explain := stringOption(flags, "explain", "e", "", "explain a check")
	baselinePath := stringOption(flags, "baseline", "b", "", "path to the check baseline")
	baselineVariant := stringOption(flags, "baseline-variant", "v", "", "generated baseline variant: loose or strict")
	generateBaseline := boolOption(flags, "generate-baseline", "g", false, "replace the baseline with all current findings")
	removeOutdated := boolOption(flags, "remove-outdated-baseline-entries", "r", false, "remove baseline entries that no longer match")
	ignoreBaseline := boolOption(flags, "ignore-baseline", "i", false, "report findings without applying a baseline")
	backupBaseline := boolOption(flags, "backup-baseline", "B", false, "back up a baseline before replacing it")
	var only stringList
	varOption(flags, &only, "only", "o", "run only these check codes (repeatable or comma-separated)")
	flags.Usage = func() {
		palette := ui.NewPalette(stderr, colorMode)
		fmt.Fprintln(stderr, palette.Accent("Usage:") + " strider check [OPTIONS] [FILE|DIR]...")
		printFlagDefaults(stderr, flags, aliases)
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
	if *watch && (*generateBaseline || *removeOutdated || *backupBaseline) {
		printCommandError(stderr, colorMode, "strider check", "--watch cannot update or back up a baseline")
		return exitError
	}

	checkConfig := configuration.Checks
	minimumSeverity, ok := resolveMinimumSeverity(flags, *minimumSeverityFlag, checkConfig.MinimumSeverity, "check", colorMode, stderr)
	if !ok {
		return exitError
	}
	registry, err := checkengine.NewRegistry(
		checkengine.RegistryOptions{
			Only: only,
			All: *allChecks,
			Settings: checkConfig.Rules,
			MinimumSeverity: minimumSeverity,
			FormatExcludes: configuration.Formatter.Excludes,
			Root: configuration.Root,
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

	baselineConfig, ok := resolveBaselineOptions(
		flags,
		configuration,
		checkConfig,
		*baselinePath,
		*baselineVariant,
		*generateBaseline,
		*removeOutdated,
		*ignoreBaseline,
		*backupBaseline,
		stderr,
		"check",
		colorMode,
	)
	if !ok {
		return exitError
	}
	baselineConfig.selectedCodes = make(map[string]bool, len(registry.Rules()))
	for _, rule := range registry.Rules() {
		baselineConfig.selectedCodes[rule.Meta().Code] = true
	}
	baselineConfig.knownCodes = registry.KnownCodes()
	workspaceOptions := workspace.Options{SkipGenerated: true, Root: configuration.Root, Excludes: checkConfig.Excludes}
	runOptions := checkengine.RunOptions{
		Formatter: formatter.Options{
			PrintWidth: configuration.Formatter.PrintWidth,
			IndentWidth: configuration.Formatter.IndentWidth,
			MaxEmptyLines: configuration.Formatter.MaxEmptyLines,
			EndOfLine: configuration.Formatter.EndOfLine,
		},
		Root: configuration.Root,
		Excludes: checkConfig.Excludes,
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
	result, err := checkengine.Run(shared, registry, runOptions)
	if err != nil {
		printCommandError(stderr, colorMode, "strider check", "%v", err)
		return exitError
	}
	diagnostics, handled, err := prepareCheckDiagnostics(result.Diagnostics, baselineConfig, colorMode, stderr)
	if err != nil {
		printCommandError(stderr, colorMode, "strider check", "%v", err)
		return exitError
	}
	if handled {
		return exitSuccess
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

const checkWatchInterval = time.Second

type checkWatcher struct {
	paths []string
	workspaceOptions workspace.Options
	cache *workspace.Cache
	session *checkengine.Session
	baseline baselineOptions
	summaryOnly bool
	colorMode ui.ColorMode
	stdout io.Writer
	stderr io.Writer

	iteration uint64
}

func runCheckWatch(
	paths []string,
	workspaceOptions workspace.Options,
	registry *checkengine.Registry,
	runOptions checkengine.RunOptions,
	baselineConfig baselineOptions,
	summaryOnly bool,
	colorMode ui.ColorMode,
	stdout,
	stderr io.Writer,
) error {
	session, err := checkengine.NewSession(registry, runOptions, checkengine.SessionOptions{})
	if err != nil {
		return err
	}
	watcher := &checkWatcher{
		paths: append([]string(nil), paths...),
		workspaceOptions: workspaceOptions,
		cache: workspace.NewCache(workspace.CacheOptions{}),
		session: session,
		baseline: baselineConfig,
		summaryOnly: summaryOnly,
		colorMode: colorMode,
		stdout: stdout,
		stderr: stderr,
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
		return checkengine.ReportJSON(stdout, diagnostics)
	}
	if reportFormat == "html" {
		return checkengine.ReportHTML(stdout, diagnostics)
	}
	if summaryOnly {
		return checkengine.ReportSummary(stdout, diagnostics, colorMode)
	}
	return checkengine.ReportText(stdout, diagnostics, colorMode)
}

func listChecksInRegistry(registry *checkengine.Registry, colorMode ui.ColorMode, stdout io.Writer) int {
	palette := ui.NewPalette(stdout, colorMode)
	entries := make([]ruleListEntry, 0, len(registry.Rules()))
	for _, rule := range registry.Rules() {
		meta := rule.Meta()
		entries = append(entries, ruleListEntry{code: meta.Code, severity: registry.Severity(meta.Code), summary: meta.Summary})
	}
	writeRuleList(stdout, palette, entries)
	return exitSuccess
}

func explainCheck(registry *checkengine.Registry, code string, colorMode ui.ColorMode, stdout, stderr io.Writer) int {
	palette := ui.NewPalette(stdout, colorMode)
	for _, rule := range registry.Rules() {
		meta := rule.Meta()
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
