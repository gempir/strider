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

	checkengine "github.com/gempir/strider/internal/check"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/ui"
	"github.com/gempir/strider/internal/workspace"
)

func runCheck(
	args []string,
	configuration config.Config,
	colorMode ui.ColorMode,
	stdout,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	reportFormat := flags.String("format", "text", "report format: text, json, or html")
	watch := flags.Bool("watch", false, "rerun checks when source changes")
	listChecks := false
	flags.BoolVar(&listChecks, "list-checks", false, "list enabled checks")
	flags.BoolVar(&listChecks, "list-rules", false, "alias for --list-checks")
	allChecks := false
	flags.BoolVar(&allChecks, "all", false, "run every built-in check")
	flags.BoolVar(&allChecks, "all-rules", false, "alias for --all")
	explain := flags.String("explain", "", "explain a check")
	baselinePath := flags.String("baseline", "", "path to the check baseline")
	baselineVariant := flags.String(
		"baseline-variant",
		"",
		"generated baseline variant: loose or strict",
	)
	generateBaseline := flags.Bool(
		"generate-baseline",
		false,
		"replace the baseline with all current findings",
	)
	removeOutdated := flags.Bool(
		"remove-outdated-baseline-entries",
		false,
		"remove baseline entries that no longer match",
	)
	ignoreBaseline := flags.Bool(
		"ignore-baseline",
		false,
		"report findings without applying a baseline",
	)
	backupBaseline := flags.Bool("backup-baseline", false, "back up a baseline before replacing it")
	var only stringList
	flags.Var(&only, "only", "run only these check codes (repeatable or comma-separated)")
	flags.Usage = func() {
		palette := ui.NewPalette(stderr, colorMode)
		fmt.Fprintln(stderr, palette.Accent("Usage:") + " strider check [OPTIONS] [FILE|DIR]...")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitError
	}
	if *reportFormat != "text" && *reportFormat != "json" && *reportFormat != "html" {
		printCommandError(
			stderr,
			colorMode,
			"strider check",
			"unsupported report format %q",
			*reportFormat,
		)
		return exitError
	}
	if *watch && *reportFormat != "text" {
		printCommandError(
			stderr,
			colorMode,
			"strider check",
			"--watch requires text report format",
		)
		return exitError
	}
	if *watch && (*generateBaseline || *removeOutdated || *backupBaseline) {
		printCommandError(
			stderr,
			colorMode,
			"strider check",
			"--watch cannot update or back up a baseline",
		)
		return exitError
	}

	checkConfig := configuration.Checks
	lintExcludes := []string(nil)
	analyzeExcludes := []string(nil)
	if configuration.Version == 1 {
		lintExcludes = configuration.EffectiveChecks(config.LegacyLintScope).Excludes
		analyzeExcludes = configuration.EffectiveChecks(config.LegacyAnalyzeScope).Excludes
	}
	registry, err := checkengine.NewRegistry(
		checkengine.RegistryOptions{
			Only: only,
			All: allChecks,
			Settings: checkConfig.Rules,
			LintExcludes: lintExcludes,
			AnalyzeExcludes: analyzeExcludes,
			FormatExcludes: configuration.Formatter.Excludes,
			Root: configuration.Root,
		},
	)
	if err != nil {
		printCommandError(stderr, colorMode, "strider check", "%v", err)
		return exitError
	}
	if listChecks {
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
	workspaceOptions := workspace.Options{
		SkipGenerated: true,
		Root: configuration.Root,
		Excludes: checkConfig.Excludes,
	}
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
		if err := runCheckWatch(
			flags.Args(),
			workspaceOptions,
			registry,
			runOptions,
			baselineConfig,
			colorMode,
			stdout,
			stderr,
		); err != nil {
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
	diagnostics, handled, err := prepareCheckDiagnostics(
		result.Diagnostics,
		baselineConfig,
		colorMode,
		stderr,
	)
	if err != nil {
		printCommandError(stderr, colorMode, "strider check", "%v", err)
		return exitError
	}
	if handled {
		return exitSuccess
	}
	err = reportCheckDiagnostics(stdout, diagnostics, *reportFormat, colorMode)
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
	diagnostics, handled, err := prepareCheckDiagnostics(
		result.Diagnostics,
		watcher.baseline,
		watcher.colorMode,
		watcher.stderr,
	)
	if err != nil {
		return err
	}
	if handled {
		return fmt.Errorf("watch mode cannot update a baseline")
	}
	watcher.iteration++
	fmt.Fprintf(watcher.stdout, "== strider check #%d ==\n", watcher.iteration)
	if err := reportCheckDiagnostics(watcher.stdout, diagnostics, "text", watcher.colorMode); err != nil {
		return err
	}
	if len(diagnostics) == 0 {
		fmt.Fprintln(watcher.stdout, "No findings.")
	}
	return nil
}

func prepareCheckDiagnostics(
	diagnostics []diagnostic.Diagnostic,
	baselineConfig baselineOptions,
	colorMode ui.ColorMode,
	stderr io.Writer,
) ([]diagnostic.Diagnostic, bool, error) {
	if baselineConfig.generate {
		diagnostics = withoutFormatDiagnostics(diagnostics)
	}
	return applyBaseline("check", diagnostics, baselineConfig, colorMode, stderr)
}

func reportCheckDiagnostics(
	stdout io.Writer,
	diagnostics []diagnostic.Diagnostic,
	reportFormat string,
	colorMode ui.ColorMode,
) error {
	if reportFormat == "json" {
		return checkengine.ReportJSON(stdout, diagnostics)
	}
	if reportFormat == "html" {
		return checkengine.ReportHTML(stdout, diagnostics)
	}
	return checkengine.ReportText(stdout, diagnostics, colorMode)
}

func listChecksInRegistry(
	registry *checkengine.Registry,
	colorMode ui.ColorMode,
	stdout io.Writer,
) int {
	palette := ui.NewPalette(stdout, colorMode)
	for _, rule := range registry.Rules() {
		meta := rule.Meta()
		fmt.Fprintf(
			stdout,
			"%s\t%s\t%s\n",
			palette.Code(meta.Code),
			colorSeverity(registry.Severity(meta.Code), palette),
			meta.Summary,
		)
	}
	return exitSuccess
}

func explainCheck(
	registry *checkengine.Registry,
	code string,
	colorMode ui.ColorMode,
	stdout,
	stderr io.Writer,
) int {
	palette := ui.NewPalette(stdout, colorMode)
	for _, rule := range registry.Rules() {
		meta := rule.Meta()
		if !strings.EqualFold(meta.Code, code) {
			continue
		}
		fmt.Fprintf(
			stdout,
			"%s (%s)\n\n%s\n\n%s\n%s\n\n%s\n%s\n",
			palette.Code(meta.Code),
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
