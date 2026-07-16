// Package app implements the Strider command-line application.
package app

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gempir/strider/internal/analyze"
	"github.com/gempir/strider/internal/baseline"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/lint"
	"github.com/gempir/strider/internal/pathfilter"
	"github.com/gempir/strider/internal/source"
	"github.com/gempir/strider/internal/ui"
)

type formattedFile struct {
	filename string
	original []byte
	result   formatter.Result
}

type formatOptions struct {
	check         bool
	diff          bool
	write         bool
	stdin         bool
	stdinFilename string
	paths         []string
	formatter     formatter.Options
	root          string
	excludes      []string
	colorMode     ui.ColorMode
}

type globalOptions struct {
	configPath string
	noConfig   bool
	color      string
	colorSet   bool
}

type baselineOptions struct {
	path     string
	variant  baseline.Variant
	generate bool
	prune    bool
	ignore   bool
	backup   bool
}

const (
	exitSuccess  = 0
	exitFindings = 1
	exitError    = 2
)

var version = "dev"

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	args, globals, ok := parseGlobalOptions(args, stderr)
	if !ok {
		return exitError
	}
	colorMode := ui.ColorAuto
	if globals.colorSet {
		colorMode = ui.ColorMode(globals.color)
	}
	if len(args) == 0 {
		usage(stderr, colorMode)
		return exitError
	}
	switch args[0] {
	case "fmt", "format":
		configuration, err := config.Load(globals.configPath, globals.noConfig)
		if err != nil {
			printError(stderr, colorMode, "strider", err)
			return exitError
		}
		colorMode = configuredColor(configuration, globals)
		return runFormat(args[1:], configuration, colorMode, stdin, stdout, stderr)
	case "lint":
		configuration, err := config.Load(globals.configPath, globals.noConfig)
		if err != nil {
			printError(stderr, colorMode, "strider", err)
			return exitError
		}
		colorMode = configuredColor(configuration, globals)
		return runLint(args[1:], configuration, colorMode, stdout, stderr)
	case "analyze":
		configuration, err := config.Load(globals.configPath, globals.noConfig)
		if err != nil {
			printError(stderr, colorMode, "strider", err)
			return exitError
		}
		colorMode = configuredColor(configuration, globals)
		return runAnalyze(args[1:], configuration, colorMode, stdout, stderr)
	case "help", "-h", "--help":
		usage(stdout, colorMode)
		return exitSuccess
	case "version", "--version":
		palette := ui.NewPalette(stdout, colorMode)
		fmt.Fprintf(stdout, "%s %s\n", palette.Bold("strider"), palette.Accent(version))
		return exitSuccess
	default:
		printError(stderr, colorMode, "strider", fmt.Errorf("unknown command %q", args[0]))
		fmt.Fprintln(stderr)
		usage(stderr, colorMode)
		return exitError
	}
}

func configuredColor(configuration config.Config, globals globalOptions) ui.ColorMode {
	if globals.colorSet {
		return ui.ColorMode(globals.color)
	}
	return ui.ColorMode(configuration.Color)
}

func parseGlobalOptions(args []string, stderr io.Writer) ([]string, globalOptions, bool) {
	options := globalOptions{}
	for len(args) != 0 {
		switch {
		case args[0] == "--config":
			if len(args) < 2 || args[1] == "" {
				printCommandError(stderr, globalColor(options), "strider", "--config requires a path")
				return nil, globalOptions{}, false
			}
			options.configPath = args[1]
			args = args[2:]
		case strings.HasPrefix(args[0], "--config="):
			options.configPath = strings.TrimPrefix(args[0], "--config=")
			if options.configPath == "" {
				printCommandError(stderr, globalColor(options), "strider", "--config requires a path")
				return nil, globalOptions{}, false
			}
			args = args[1:]
		case args[0] == "--no-config":
			options.noConfig = true
			args = args[1:]
		case args[0] == "--color" || args[0] == "--colors":
			if len(args) < 2 || !ui.ValidColorMode(args[1]) {
				printCommandError(stderr, globalColor(options), "strider", "--color must be auto, always, or never")
				return nil, globalOptions{}, false
			}
			options.color = args[1]
			options.colorSet = true
			args = args[2:]
		case strings.HasPrefix(args[0], "--color=") || strings.HasPrefix(args[0], "--colors="):
			_, value, _ := strings.Cut(args[0], "=")
			if !ui.ValidColorMode(value) {
				printCommandError(stderr, globalColor(options), "strider", "--color must be auto, always, or never")
				return nil, globalOptions{}, false
			}
			options.color = value
			options.colorSet = true
			args = args[1:]
		default:
			if options.configPath != "" && options.noConfig {
				printCommandError(stderr, globalColor(options), "strider", "--config and --no-config are mutually exclusive")
				return nil, globalOptions{}, false
			}
			return args, options, true
		}
	}
	return args, options, true
}

func globalColor(options globalOptions) ui.ColorMode {
	if options.colorSet {
		return ui.ColorMode(options.color)
	}
	return ui.ColorAuto
}

func usage(writer io.Writer, colorMode ui.ColorMode) {
	palette := ui.NewPalette(writer, colorMode)
	fmt.Fprintln(writer, palette.Bold("Strider")+" formats, lints, and statically analyzes Go code.")
	fmt.Fprintf(writer, "\n%s\n", palette.Accent("Usage:"))
	fmt.Fprintf(writer, "  %s [--config PATH|--no-config] [--color auto|always|never] COMMAND [OPTIONS]\n", palette.Bold("strider"))
	fmt.Fprintf(writer, "\n%s\n", palette.Accent("Commands:"))
	fmt.Fprintf(writer, "  %s       Format Go source (alias: format)\n", palette.Code("fmt"))
	fmt.Fprintf(writer, "  %s      Run clarity and safety rules\n", palette.Code("lint"))
	fmt.Fprintf(writer, "  %s   Run package-aware static analysis\n", palette.Code("analyze"))
	fmt.Fprintf(writer, "  %s   Print the version\n", palette.Code("version"))
}

func printError(writer io.Writer, colorMode ui.ColorMode, command string, err error) {
	palette := ui.NewPalette(writer, colorMode)
	fmt.Fprintf(writer, "%s %s\n", palette.Error(command+":"), err)
}

func printCommandError(
	writer io.Writer,
	colorMode ui.ColorMode,
	command, format string,
	arguments ...any,
) {
	palette := ui.NewPalette(writer, colorMode)
	fmt.Fprintf(writer, "%s %s\n", palette.Error(command+":"), fmt.Sprintf(format, arguments...))
}

func runFormat(
	args []string,
	configuration config.Config,
	colorMode ui.ColorMode,
	stdin io.Reader,
	stdout, stderr io.Writer,
) int {
	options, ok := parseFormatOptions(args, colorMode, stderr)
	if !ok {
		return exitError
	}
	options.formatter = formatter.Options{
		PrintWidth:    configuration.Formatter.PrintWidth,
		IndentWidth:   configuration.Formatter.IndentWidth,
		MaxEmptyLines: configuration.Formatter.MaxEmptyLines,
		EndOfLine:     configuration.Formatter.EndOfLine,
	}
	options.root = configuration.Root
	options.excludes = configuration.Formatter.Excludes
	options.colorMode = colorMode
	if options.stdin {
		return formatStdin(options.stdinFilename, options.formatter, colorMode, stdin, stdout, stderr)
	}
	return formatPaths(options, stdout, stderr)
}

func parseFormatOptions(args []string, colorMode ui.ColorMode, stderr io.Writer) (formatOptions, bool) {
	flags := flag.NewFlagSet("fmt", flag.ContinueOnError)
	flags.SetOutput(stderr)
	check := flags.Bool("check", false, "report files that would change without writing")
	diffMode := flags.Bool("diff", false, "print full unified diffs without writing")
	write := flags.Bool("write", false, "write formatted source in place")
	stdinMode := flags.Bool("stdin", false, "read source from stdin and write it to stdout")
	stdinFilename := flags.String("stdin-filename", "<stdin>", "logical filename for stdin")
	flags.Usage = func() {
		palette := ui.NewPalette(stderr, colorMode)
		fmt.Fprintln(stderr, palette.Accent("Usage:")+" strider fmt [--check|--diff|--write|--stdin] [FILE|DIR]...")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return formatOptions{}, false
	}
	stdinFilenameSet := flagWasSet(flags, "stdin-filename")
	if stdinFilenameSet && !*stdinMode {
		printCommandError(stderr, colorMode, "strider fmt", "--stdin-filename requires --stdin")
		return formatOptions{}, false
	}
	modeCount := boolInt(*check) + boolInt(*diffMode) + boolInt(*write)
	if modeCount > 1 {
		printCommandError(stderr, colorMode, "strider fmt", "--check, --diff, and --write are mutually exclusive")
		return formatOptions{}, false
	}
	paths := flags.Args()
	if *stdinMode {
		if len(paths) != 0 {
			printCommandError(stderr, colorMode, "strider fmt", "--stdin does not accept file or directory arguments")
			return formatOptions{}, false
		}
		if modeCount != 0 {
			printCommandError(stderr, colorMode, "strider fmt", "formatting stdin does not accept --check, --diff, or --write")
			return formatOptions{}, false
		}
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}
	if modeCount == 0 {
		*write = true
	}
	return formatOptions{
		check:         *check,
		diff:          *diffMode,
		write:         *write,
		stdin:         *stdinMode,
		stdinFilename: *stdinFilename,
		paths:         paths,
	}, true
}

func flagWasSet(flags *flag.FlagSet, name string) bool {
	found := false
	flags.Visit(
		func(current *flag.Flag) {
			if current.Name == name {
				found = true
			}
		},
	)
	return found
}

func formatStdin(
	filename string,
	formatOptions formatter.Options,
	colorMode ui.ColorMode,
	stdin io.Reader,
	stdout, stderr io.Writer,
) int {
	input, err := io.ReadAll(stdin)
	if err != nil {
		printCommandError(stderr, colorMode, "strider fmt", "%v", err)
		return exitError
	}
	result, err := formatter.FormatWithOptions(filename, input, formatOptions)
	if err != nil {
		printCommandError(stderr, colorMode, "strider fmt", "%v", err)
		return exitError
	}
	if _, err := stdout.Write(result.Source); err != nil {
		printCommandError(stderr, colorMode, "strider fmt", "%v", err)
		return exitError
	}
	return exitSuccess
}

func formatPaths(options formatOptions, stdout, stderr io.Writer) int {
	files, err := source.Discover(options.paths, source.Options{SkipGenerated: true})
	if err != nil {
		printCommandError(stderr, options.colorMode, "strider fmt", "%v", err)
		return exitError
	}
	files = filterFiles(files, options.root, options.excludes)
	formatted := make([]formattedFile, 0, len(files))
	for _, filename := range files {
		original, readErr := os.ReadFile(filename)
		if readErr != nil {
			printCommandError(stderr, options.colorMode, "strider fmt", "%s: %v", source.DisplayPath(filename), readErr)
			return exitError
		}
		result, formatErr := formatter.FormatWithOptions(filename, original, options.formatter)
		if formatErr != nil {
			printCommandError(stderr, options.colorMode, "strider fmt", "%v", formatErr)
			return exitError
		}
		formatted = append(
			formatted,
			formattedFile{filename: filename, original: original, result: result},
		)
	}
	changed := reportFormatChanges(formatted, options, stdout)
	if options.write {
		if err := atomicWriteBatch(formatted); err != nil {
			printCommandError(stderr, options.colorMode, "strider fmt", "%v", err)
			return exitError
		}
		return exitSuccess
	}
	if changed {
		return exitFindings
	}
	return exitSuccess
}

func reportFormatChanges(files []formattedFile, options formatOptions, stdout io.Writer) bool {
	palette := ui.NewPalette(stdout, options.colorMode)
	changed := false
	for _, file := range files {
		if !file.result.Changed {
			continue
		}
		changed = true
		switch {
		case options.check:
			fmt.Fprintf(stdout, "%s %s\n", palette.Warning("would reformat"), palette.Path(source.DisplayPath(file.filename)))
		case options.diff:
			printDiff(stdout, source.DisplayPath(file.filename), file.original, file.result.Source, palette)
		}
	}
	return changed
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

type stagedFile struct {
	temporary string
	target    string
}

func atomicWriteBatch(files []formattedFile) error {
	staged := []stagedFile{}
	cleanup := func() {
		for _, file := range staged {
			_ = os.Remove(file.temporary)
		}
	}
	for _, file := range files {
		if !file.result.Changed {
			continue
		}
		stagedFile, err := stageFile(file)
		if err != nil {
			cleanup()
			return err
		}
		staged = append(staged, stagedFile)
	}
	for index, file := range staged {
		if err := os.Rename(file.temporary, file.target); err != nil {
			for _, remaining := range staged[index:] {
				_ = os.Remove(remaining.temporary)
			}
			return err
		}
	}
	return nil
}

func stageFile(file formattedFile) (stagedFile, error) {
	info, err := os.Stat(file.filename)
	if err != nil {
		return stagedFile{}, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(file.filename), ".strider-*")
	if err != nil {
		return stagedFile{}, err
	}
	name := temporary.Name()
	if err = temporary.Chmod(info.Mode().Perm()); err == nil {
		_, err = temporary.Write(file.result.Source)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(name)
		return stagedFile{}, err
	}
	return stagedFile{temporary: name, target: file.filename}, nil
}

func printDiff(writer io.Writer, filename string, before, after []byte, palette ui.Palette) {
	beforeLines := splitLines(before)
	afterLines := splitLines(after)
	fmt.Fprintln(writer, palette.Removed("--- "+filename))
	fmt.Fprintln(writer, palette.Added("+++ "+filename))
	fmt.Fprintln(writer, palette.Hunk(fmt.Sprintf("@@ -1,%d +1,%d @@", len(beforeLines), len(afterLines))))
	for _, line := range beforeLines {
		fmt.Fprintln(writer, palette.Removed("-"+line))
	}
	for _, line := range afterLines {
		fmt.Fprintln(writer, palette.Added("+"+line))
	}
}

func splitLines(content []byte) []string {
	content = bytes.TrimSuffix(content, []byte("\n"))
	if len(content) == 0 {
		return nil
	}
	return strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
}

type stringList []string

func (values *stringList) String() string {
	return strings.Join(*values, ",")
}

func (values *stringList) Set(value string) error {
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			*values = append(*values, item)
		}
	}
	return nil
}

func runLint(
	args []string,
	configuration config.Config,
	colorMode ui.ColorMode,
	stdout, stderr io.Writer,
) int {
	flags := flag.NewFlagSet("lint", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "text", "report format: text or json")
	listRules := flags.Bool("list-rules", false, "list enabled lint rules")
	allRules := flags.Bool("all-rules", false, "run every built-in rule")
	explain := flags.String("explain", "", "explain a lint rule")
	baselinePath := flags.String("baseline", "", "path to the lint baseline")
	baselineVariant := flags.String("baseline-variant", "", "generated baseline variant: loose or strict")
	generateBaseline := flags.Bool("generate-baseline", false, "replace the baseline with all current findings")
	removeOutdated := flags.Bool(
		"remove-outdated-baseline-entries",
		false,
		"remove baseline entries that no longer match",
	)
	ignoreBaseline := flags.Bool("ignore-baseline", false, "report findings without applying a baseline")
	backupBaseline := flags.Bool("backup-baseline", false, "back up a baseline before replacing it")
	var only stringList
	flags.Var(&only, "only", "run only these rule codes (repeatable or comma-separated)")
	flags.Usage = func() {
		palette := ui.NewPalette(stderr, colorMode)
		fmt.Fprintln(stderr, palette.Accent("Usage:")+" strider lint [OPTIONS] [FILE|DIR]...")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitError
	}
	if *allRules && len(only) != 0 {
		printCommandError(stderr, colorMode, "strider lint", "--all-rules and --only are mutually exclusive")
		return exitError
	}
	baselineConfig, ok := resolveBaselineOptions(
		flags,
		configuration,
		configuration.Linter,
		*baselinePath,
		*baselineVariant,
		*generateBaseline,
		*removeOutdated,
		*ignoreBaseline,
		*backupBaseline,
		stderr,
		"lint",
		colorMode,
	)
	if !ok {
		return exitError
	}
	var registry *lint.Registry
	var err error
	registry, err = lint.NewRegistryConfigured(
		only,
		*allRules,
		configuration.Linter.Rules,
		configuration.Root,
	)
	if err != nil {
		printCommandError(stderr, colorMode, "strider lint", "%v", err)
		return exitError
	}
	if *listRules {
		return listLintRules(registry, colorMode, stdout)
	}
	if *explain != "" {
		return explainLintRule(registry, *explain, colorMode, stdout, stderr)
	}
	if *format != "text" && *format != "json" {
		printCommandError(stderr, colorMode, "strider lint", "unsupported report format %q", *format)
		return exitError
	}
	return lintPaths(
		flags.Args(),
		*format,
		registry,
		configuration.Root,
		configuration.Linter.Excludes,
		baselineConfig,
		colorMode,
		stdout,
		stderr,
	)
}

func listLintRules(registry *lint.Registry, colorMode ui.ColorMode, stdout io.Writer) int {
	palette := ui.NewPalette(stdout, colorMode)
	rules := registry.Rules()
	sort.Slice(
		rules,
		func(i, j int) bool {
			return rules[i].Meta().Code < rules[j].Meta().Code
		},
	)
	for _, rule := range rules {
		meta := rule.Meta()
		severity := registry.Severity(meta.Code)
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", palette.Code(meta.Code), colorSeverity(severity, palette), meta.Summary)
	}
	return exitSuccess
}

func explainLintRule(
	registry *lint.Registry,
	code string,
	colorMode ui.ColorMode,
	stdout, stderr io.Writer,
) int {
	palette := ui.NewPalette(stdout, colorMode)
	for _, rule := range registry.Rules() {
		meta := rule.Meta()
		if meta.Code != code {
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
	printCommandError(stderr, colorMode, "strider lint", "unknown lint rule %q", code)
	return exitError
}

func lintPaths(
	paths []string,
	format string,
	registry *lint.Registry,
	root string,
	excludes []string,
	baselineConfig baselineOptions,
	colorMode ui.ColorMode,
	stdout, stderr io.Writer,
) int {
	files, err := source.Discover(paths, source.Options{SkipGenerated: true})
	if err != nil {
		printCommandError(stderr, colorMode, "strider lint", "%v", err)
		return exitError
	}
	files = filterFiles(files, root, excludes)
	diagnostics, err := lint.Run(files, registry)
	if err != nil {
		printCommandError(stderr, colorMode, "strider lint", "%v", err)
		return exitError
	}
	diagnostics, handled, err := applyBaseline("lint", diagnostics, baselineConfig, colorMode, stderr)
	if err != nil {
		printCommandError(stderr, colorMode, "strider lint", "%v", err)
		return exitError
	}
	if handled {
		return exitSuccess
	}
	if format == "json" {
		err = lint.ReportJSON(stdout, diagnostics)
	} else {
		err = lint.ReportText(stdout, diagnostics, colorMode)
	}
	if err != nil {
		printCommandError(stderr, colorMode, "strider lint", "%v", err)
		return exitError
	}
	if len(diagnostics) != 0 {
		return exitFindings
	}
	return exitSuccess
}

func runAnalyze(
	args []string,
	configuration config.Config,
	colorMode ui.ColorMode,
	stdout, stderr io.Writer,
) int {
	flags := flag.NewFlagSet("analyze", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "text", "report format: text or json")
	listRules := flags.Bool("list-rules", false, "list enabled analysis rules")
	explain := flags.String("explain", "", "explain an analysis rule")
	baselinePath := flags.String("baseline", "", "path to the analysis baseline")
	baselineVariant := flags.String("baseline-variant", "", "generated baseline variant: loose or strict")
	generateBaseline := flags.Bool("generate-baseline", false, "replace the baseline with all current findings")
	removeOutdated := flags.Bool(
		"remove-outdated-baseline-entries",
		false,
		"remove baseline entries that no longer match",
	)
	ignoreBaseline := flags.Bool("ignore-baseline", false, "report findings without applying a baseline")
	backupBaseline := flags.Bool("backup-baseline", false, "back up a baseline before replacing it")
	var only stringList
	flags.Var(&only, "only", "run only these rule codes (repeatable or comma-separated)")
	flags.Usage = func() {
		palette := ui.NewPalette(stderr, colorMode)
		fmt.Fprintln(stderr, palette.Accent("Usage:")+" strider analyze [OPTIONS] [FILE|DIR]...")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitError
	}
	baselineConfig, ok := resolveBaselineOptions(
		flags,
		configuration,
		configuration.Analyzer,
		*baselinePath,
		*baselineVariant,
		*generateBaseline,
		*removeOutdated,
		*ignoreBaseline,
		*backupBaseline,
		stderr,
		"analyze",
		colorMode,
	)
	if !ok {
		return exitError
	}
	registry, err := analyze.NewRegistryConfigured(only, configuration.Analyzer.Rules, configuration.Root)
	if err != nil {
		printCommandError(stderr, colorMode, "strider analyze", "%v", err)
		return exitError
	}
	if *listRules {
		return listAnalyzeRules(registry, colorMode, stdout)
	}
	if *explain != "" {
		return explainAnalyzeRule(registry, *explain, colorMode, stdout, stderr)
	}
	if *format != "text" && *format != "json" {
		printCommandError(stderr, colorMode, "strider analyze", "unsupported report format %q", *format)
		return exitError
	}
	return analyzePaths(
		flags.Args(),
		*format,
		registry,
		configuration.Root,
		configuration.Analyzer.Excludes,
		baselineConfig,
		colorMode,
		stdout,
		stderr,
	)
}

func listAnalyzeRules(registry *analyze.Registry, colorMode ui.ColorMode, stdout io.Writer) int {
	palette := ui.NewPalette(stdout, colorMode)
	rules := registry.Rules()
	sort.Slice(
		rules,
		func(i, j int) bool {
			return rules[i].Meta().Code < rules[j].Meta().Code
		},
	)
	for _, rule := range rules {
		meta := rule.Meta()
		severity := registry.Severity(meta.Code)
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", palette.Code(meta.Code), colorSeverity(severity, palette), meta.Summary)
	}
	return exitSuccess
}

func explainAnalyzeRule(
	registry *analyze.Registry,
	code string,
	colorMode ui.ColorMode,
	stdout, stderr io.Writer,
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
	printCommandError(stderr, colorMode, "strider analyze", "unknown analysis rule %q", code)
	return exitError
}

func analyzePaths(
	paths []string,
	format string,
	registry *analyze.Registry,
	root string,
	excludes []string,
	baselineConfig baselineOptions,
	colorMode ui.ColorMode,
	stdout, stderr io.Writer,
) int {
	diagnostics, err := analyze.Run(paths, registry)
	if err != nil {
		printCommandError(stderr, colorMode, "strider analyze", "%v", err)
		return exitError
	}
	diagnostics = filterDiagnostics(diagnostics, root, excludes)
	diagnostics, handled, err := applyBaseline("analyze", diagnostics, baselineConfig, colorMode, stderr)
	if err != nil {
		printCommandError(stderr, colorMode, "strider analyze", "%v", err)
		return exitError
	}
	if handled {
		return exitSuccess
	}
	if format == "json" {
		err = analyze.ReportJSON(stdout, diagnostics)
	} else {
		err = analyze.ReportText(stdout, diagnostics, colorMode)
	}
	if err != nil {
		printCommandError(stderr, colorMode, "strider analyze", "%v", err)
		return exitError
	}
	if len(diagnostics) != 0 {
		return exitFindings
	}
	return exitSuccess
}

func colorSeverity(severity diagnostic.Severity, palette ui.Palette) string {
	switch severity {
	case diagnostic.SeverityError:
		return palette.Error(string(severity))
	case diagnostic.SeverityNote:
		return palette.Note(string(severity))
	default:
		return palette.Warning(string(severity))
	}
}

func resolveBaselineOptions(
	flags *flag.FlagSet,
	configuration config.Config,
	tool config.ToolConfig,
	path, variant string,
	generate, prune, ignore, backup bool,
	stderr io.Writer,
	command string,
	colorMode ui.ColorMode,
) (baselineOptions, bool) {
	if !flagWasSet(flags, "baseline") {
		path = configuration.Resolve(tool.Baseline)
	}
	if !flagWasSet(flags, "baseline-variant") {
		variant = tool.BaselineVariant
	}
	if variant != "loose" && variant != "strict" {
		printCommandError(stderr, colorMode, "strider "+command, "--baseline-variant must be loose or strict")
		return baselineOptions{}, false
	}
	if generate && prune {
		printCommandError(stderr, colorMode, "strider "+command, "--generate-baseline and --remove-outdated-baseline-entries are mutually exclusive")
		return baselineOptions{}, false
	}
	if ignore && (generate || prune) {
		printCommandError(stderr, colorMode, "strider "+command, "--ignore-baseline cannot be combined with baseline updates")
		return baselineOptions{}, false
	}
	if backup && !generate && !prune {
		printCommandError(stderr, colorMode, "strider "+command, "--backup-baseline requires a baseline update option")
		return baselineOptions{}, false
	}
	if path == "" && (generate || prune) {
		printCommandError(stderr, colorMode, "strider "+command, "baseline update requires --baseline or a configured baseline")
		return baselineOptions{}, false
	}
	if path != "" {
		absolute, err := filepath.Abs(path)
		if err != nil {
			printCommandError(stderr, colorMode, "strider "+command, "baseline path: %v", err)
			return baselineOptions{}, false
		}
		path = absolute
	}
	return baselineOptions{
		path: path, variant: baseline.Variant(variant), generate: generate,
		prune: prune, ignore: ignore, backup: backup,
	}, true
}

func applyBaseline(
	command string,
	diagnostics []diagnostic.Diagnostic,
	options baselineOptions,
	colorMode ui.ColorMode,
	stderr io.Writer,
) ([]diagnostic.Diagnostic, bool, error) {
	if options.path == "" || options.ignore {
		return diagnostics, false, nil
	}
	if options.generate {
		generated, err := baseline.Generate(options.path, options.variant, diagnostics)
		if err != nil {
			return nil, false, err
		}
		if err := baseline.Write(options.path, generated, options.backup); err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}
	loaded, err := baseline.Load(options.path)
	if err != nil {
		return nil, false, fmt.Errorf("load baseline %s: %w", options.path, err)
	}
	result, err := baseline.Apply(options.path, loaded, diagnostics)
	if err != nil {
		return nil, false, err
	}
	if options.prune {
		if err := baseline.Write(options.path, result.Matched, options.backup); err != nil {
			return nil, false, err
		}
	} else if result.Stale != 0 {
		palette := ui.NewPalette(stderr, colorMode)
		fmt.Fprintf(
			stderr,
			"%s baseline has %d outdated issue(s); run with %s to prune them\n",
			palette.Warning("strider "+command+":"),
			result.Stale,
			palette.Code("--remove-outdated-baseline-entries"),
		)
	}
	return result.Diagnostics, false, nil
}

func filterFiles(files []string, root string, excludes []string) []string {
	filtered := make([]string, 0, len(files))
	for _, filename := range files {
		if !pathfilter.Matches(root, filename, excludes) {
			filtered = append(filtered, filename)
		}
	}
	return filtered
}

func filterDiagnostics(
	diagnostics []diagnostic.Diagnostic,
	root string,
	excludes []string,
) []diagnostic.Diagnostic {
	filtered := make([]diagnostic.Diagnostic, 0, len(diagnostics))
	for _, item := range diagnostics {
		if !pathfilter.Matches(root, item.File, excludes) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
