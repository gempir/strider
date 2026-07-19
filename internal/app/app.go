// Package app implements the Strider command-line application.
package app

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/gempir/strider/internal/baseline"
	checkengine "github.com/gempir/strider/internal/checks"
	"github.com/gempir/strider/internal/checks/semantic"
	"github.com/gempir/strider/internal/checks/syntax"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/pathfilter"
	"github.com/gempir/strider/internal/source"
	"github.com/gempir/strider/internal/ui"
	"github.com/gempir/strider/internal/workspace"
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
	path          string
	generate      bool
	prune         bool
	selectedCodes map[string]bool
	knownCodes    map[string]bool
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
	case "check":
		configuration, err := config.Load(globals.configPath, globals.noConfig)
		if err != nil {
			printError(stderr, colorMode, "strider", err)
			return exitError
		}
		colorMode = configuredColor(configuration, globals)
		return runCheck(args[1:], configuration, colorMode, stdout, stderr)
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
	case "version", "-v", "--version":
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
		case args[0] == "--config" || args[0] == "-c":
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
		case strings.HasPrefix(args[0], "-c="):
			options.configPath = strings.TrimPrefix(args[0], "-c=")
			if options.configPath == "" {
				printCommandError(stderr, globalColor(options), "strider", "--config requires a path")
				return nil, globalOptions{}, false
			}
			args = args[1:]
		case args[0] == "--no-config" || args[0] == "-n":
			options.noConfig = true
			args = args[1:]
		case args[0] == "--color" || args[0] == "--colors" || args[0] == "-C":
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
		case strings.HasPrefix(args[0], "-C="):
			value := strings.TrimPrefix(args[0], "-C=")
			if !ui.ValidColorMode(value) {
				printCommandError(stderr, globalColor(options), "strider", "--color must be auto, always, or never")
				return nil, globalOptions{}, false
			}
			options.color = value
			options.colorSet = true
			args = args[1:]
		case strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[0], "--") && len(args[0]) > 2:
			name := strings.TrimPrefix(strings.SplitN(args[0], "=", 2)[0], "-")
			aliases := map[string]string{"config": "c", "no-config": "n", "color": "C", "colors": "C", "help": "h", "version": "v"}
			replacement := "--" + name
			if short := aliases[name]; short != "" {
				replacement += " or -" + short
			}
			printCommandError(stderr, globalColor(options), "strider", "long option %q must use two dashes; use %s", args[0], replacement)
			return nil, globalOptions{}, false
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
	fmt.Fprintln(writer, palette.Bold("Strider")+" formats and checks Go code.")
	fmt.Fprintf(writer, "\n%s\n", palette.Accent("Usage:"))
	fmt.Fprintf(writer, "  %s [-c PATH|--config PATH|-n|--no-config] [-C MODE|--color MODE] COMMAND [OPTIONS]\n", palette.Bold("strider"))
	fmt.Fprintf(writer, "\n%s\n", palette.Accent("Commands:"))
	fmt.Fprintf(writer, "  %s       Format Go source (alias: format)\n", palette.Code("fmt"))
	fmt.Fprintf(writer, "  %s     Run formatting, clarity, and correctness checks\n", palette.Code("check"))
	fmt.Fprintf(writer, "  %s   Print the version\n", palette.Code("version"))
}

func printError(writer io.Writer, colorMode ui.ColorMode, command string, err error) {
	palette := ui.NewPalette(writer, colorMode)
	fmt.Fprintf(writer, "%s %s\n", palette.Error(command+":"), err)
}

func printCommandError(writer io.Writer, colorMode ui.ColorMode, command, format string, arguments ...any) {
	palette := ui.NewPalette(writer, colorMode)
	fmt.Fprintf(writer, "%s %s\n", palette.Error(command+":"), fmt.Sprintf(format, arguments...))
}

func runFormat(args []string, configuration config.Config, colorMode ui.ColorMode, stdin io.Reader, stdout, stderr io.Writer) int {
	options, ok := parseFormatOptions(args, colorMode, stderr)
	if !ok {
		return exitError
	}
	options.formatter = formatter.Options{PrintWidth: configuration.Formatter.PrintWidth}
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
	aliases := map[string]string{"check": "c", "diff": "d", "write": "w", "stdin": "s", "stdin-filename": "f", "help": "h"}
	check := boolOption(flags, "check", "c", false, "report files that would change without writing")
	diffMode := boolOption(flags, "diff", "d", false, "print full unified diffs without writing")
	write := boolOption(flags, "write", "w", false, "write formatted source in place")
	stdinMode := boolOption(flags, "stdin", "s", false, "read source from stdin and write it to stdout")
	stdinFilename := stringOption(flags, "stdin-filename", "f", "<stdin>", "logical filename for stdin")
	flags.Usage = func() {
		palette := ui.NewPalette(stderr, colorMode)
		fmt.Fprintln(stderr, palette.Accent("Usage:")+" strider fmt [--check|--diff|--write|--stdin] [FILE|DIR]...")
		printFlagDefaults(stderr, flags, aliases, palette)
	}
	if !parseCommandFlags(flags, args, aliases, "fmt", colorMode, stderr) {
		return formatOptions{}, false
	}
	stdinFilenameSet := flagWasSetAny(flags, "stdin-filename", "f")
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
	return formatOptions{check: *check, diff: *diffMode, write: *write, stdin: *stdinMode, stdinFilename: *stdinFilename, paths: paths}, true
}

func flagWasSet(flags *flag.FlagSet, name string) bool {
	found := false
	flags.Visit(func(current *flag.Flag) {
		if current.Name == name {
			found = true
		}
	})
	return found
}

func flagWasSetAny(flags *flag.FlagSet, names ...string) bool {
	for _, name := range names {
		if flagWasSet(flags, name) {
			return true
		}
	}
	return false
}

func stringOption(flags *flag.FlagSet, long, short, fallback, usage string) *string {
	value := fallback
	flags.StringVar(&value, long, fallback, usage)
	flags.StringVar(&value, short, fallback, "alias for --"+long)
	return &value
}

func boolOption(flags *flag.FlagSet, long, short string, fallback bool, usage string) *bool {
	value := fallback
	flags.BoolVar(&value, long, fallback, usage)
	flags.BoolVar(&value, short, fallback, "alias for --"+long)
	return &value
}

func varOption(flags *flag.FlagSet, value flag.Value, long, short, usage string) {
	flags.Var(value, long, usage)
	flags.Var(value, short, "alias for --"+long)
}

func checkCommandAliases() map[string]string {
	aliases := map[string]string{
		"format":                           "f",
		"minimum-severity":                 "s",
		"list-rules":                       "l",
		"explain":                          "e",
		"baseline":                         "b",
		"generate-baseline":                "g",
		"remove-outdated-baseline-entries": "r",
		"only":                             "o",
		"help":                             "h",
	}
	return aliases
}

func parseCommandFlags(flags *flag.FlagSet, args []string, aliases map[string]string, command string, colorMode ui.ColorMode, stderr io.Writer) bool {
	for _, argument := range args {
		if argument == "--" {
			break
		}
		if !strings.HasPrefix(argument, "-") || strings.HasPrefix(argument, "--") || len(argument) <= 2 || isShortOptionAssignment(argument, aliases) {
			continue
		}
		name := strings.TrimPrefix(strings.SplitN(argument, "=", 2)[0], "-")
		short := aliases[name]
		replacement := "--" + name
		if short != "" {
			replacement += " or -" + short
		}
		printCommandError(stderr, colorMode, "strider "+command, "long option %q must use two dashes; use %s", argument, replacement)
		return false
	}
	return flags.Parse(args) == nil
}

func isShortOptionAssignment(argument string, aliases map[string]string) bool {
	if len(argument) < 4 || argument[0] != '-' || argument[2] != '=' {
		return false
	}
	short := argument[1:2]
	for _, alias := range aliases {
		if alias == short {
			return true
		}
	}
	return false
}

func printFlagDefaults(writer io.Writer, flags *flag.FlagSet, aliases map[string]string, palette ui.Palette) {
	flags.VisitAll(
		func(option *flag.Flag) {
			if len(option.Name) == 1 {
				return
			}
			if option.Name == "list-rules" {
				return
			}
			value := " VALUE"
			if boolean,
				ok := option.Value.(interface {
				IsBoolFlag() bool
			}); ok && boolean.IsBoolFlag() {
				value = ""
			}
			short := "    "
			if alias := aliases[option.Name]; alias != "" {
				short = "-" + alias + ", "
			}
			usage := option.Usage
			if option.DefValue != "" && option.DefValue != "false" {
				usage += fmt.Sprintf(" (default %q)", option.DefValue)
			}
			fmt.Fprintf(writer, "  %s%s%s\n      %s\n", palette.Code(short), palette.Code("--"+option.Name), palette.Muted(value), usage)
		},
	)
}

func formatStdin(filename string, formatOptions formatter.Options, colorMode ui.ColorMode, stdin io.Reader, stdout, stderr io.Writer) int {
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
	shared, err := workspace.Open(options.paths, workspace.Options{SkipGenerated: true, Root: options.root, Excludes: options.excludes})
	if err != nil {
		printCommandError(stderr, options.colorMode, "strider fmt", "%v", err)
		return exitError
	}
	formatted, formatErrors := formatFiles(shared.Files(), options.formatter, options.write || options.diff)
	for _, formatErr := range formatErrors {
		if formatErr != nil {
			printCommandError(stderr, options.colorMode, "strider fmt", "%v", formatErr)
			return exitError
		}
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

func formatFiles(files []*workspace.File, options formatter.Options, verify bool) ([]formattedFile, []error) {
	formatted := make([]formattedFile, len(files))
	errorsByFile := make([]error, len(files))
	if len(files) == 0 {
		return formatted, errorsByFile
	}

	session := formatter.NewSession()
	jobs := make(chan int)
	workers := min(runtime.GOMAXPROCS(0), len(files))
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := range jobs {
				file := files[index]
				func() {
					defer file.Release()
					filename := file.Path()
					original, err := file.Bytes()
					if err != nil {
						errorsByFile[index] = fmt.Errorf("%s: %w", source.DisplayPath(filename), err)
						return
					}
					if formatter.IsIgnored(original) {
						formatted[index] = formattedFile{
							filename: filename,
							original: original,
							result:   formatter.Result{Source: append([]byte(nil), original...), Ignored: true},
						}
						return
					}
					tree, err := file.CST()
					if err != nil {
						errorsByFile[index] = fmt.Errorf("%s: %w", source.DisplayPath(filename), err)
						return
					}
					var result formatter.Result
					if verify {
						result, err = session.FormatTree(filename, tree, options)
					} else {
						result, err = session.PreviewTree(filename, tree, options)
					}
					if err != nil {
						errorsByFile[index] = err
						return
					}
					formatted[index] = formattedFile{filename: filename, original: original, result: result}
				}()
			}
		}()
	}
	for index := range files {
		jobs <- index
	}
	close(jobs)
	group.Wait()
	return formatted, errorsByFile
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
	staged := make([]stagedFile, 0, len(files))
	cleanup := func() error {
		var cleanupErr error
		for _, file := range staged {
			cleanupErr = errors.Join(cleanupErr, os.Remove(file.temporary))
		}
		return cleanupErr
	}
	for _, file := range files {
		if !file.result.Changed {
			continue
		}
		stagedFile, err := stageFile(file)
		if err != nil {
			return errors.Join(err, cleanup())
		}
		staged = append(staged, stagedFile)
	}
	for index, file := range staged {
		if err := os.Rename(file.temporary, file.target); err != nil {
			var cleanupErr error
			for _, remaining := range staged[index:] {
				cleanupErr = errors.Join(cleanupErr, os.Remove(remaining.temporary))
			}
			return errors.Join(err, cleanupErr)
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
		return stagedFile{}, errors.Join(err, os.Remove(name))
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

func runLint(args []string, configuration config.Config, colorMode ui.ColorMode, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("lint", flag.ContinueOnError)
	flags.SetOutput(stderr)
	aliases := checkCommandAliases()
	format := stringOption(flags, "format", "f", "text", "report format: text, json, or html")
	minimumSeverityFlag := stringOption(flags, "minimum-severity", "s", "", "minimum effective severity: none, note, warning, or error")
	listRules := boolOption(flags, "list-rules", "l", false, "list lint rules admitted by the severity floor")
	explain := stringOption(flags, "explain", "e", "", "explain a lint rule")
	baselinePath := stringOption(flags, "baseline", "b", "", "path to the lint baseline")
	generateBaseline := boolOption(flags, "generate-baseline", "g", false, "replace the baseline with all current findings")
	removeOutdated := boolOption(flags, "remove-outdated-baseline-entries", "r", false, "remove baseline entries that no longer match")
	var only stringList
	varOption(flags, &only, "only", "o", "run only these rule codes (repeatable or comma-separated)")
	flags.Usage = func() {
		palette := ui.NewPalette(stderr, colorMode)
		fmt.Fprintln(stderr, palette.Accent("Usage:")+" strider lint [OPTIONS] [FILE|DIR]...")
		printFlagDefaults(stderr, flags, aliases, palette)
	}
	if !parseCommandFlags(flags, args, aliases, "lint", colorMode, stderr) {
		return exitError
	}
	lintConfig := configuration.Checks
	syntaxSettings, knownCodes, settingsErr := commandSettings(lintConfig.Rules, checkengine.CapabilityCST)
	if settingsErr != nil {
		printCommandError(stderr, colorMode, "strider lint", "%v", settingsErr)
		return exitError
	}
	minimumSeverity, ok := resolveMinimumSeverity(flags, *minimumSeverityFlag, lintConfig.MinimumSeverity, "lint", colorMode, stderr)
	if !ok {
		return exitError
	}
	baselineConfig, ok := resolveBaselineOptions(flags, configuration, lintConfig, *baselinePath, *generateBaseline, *removeOutdated, stderr, "lint", colorMode)
	if !ok {
		return exitError
	}
	var registry *syntax.Registry
	var err error
	registry, err = syntax.NewRegistryWithOptions(syntax.RegistryOptions{Only: only, Settings: syntaxSettings, Root: configuration.Root, MinimumSeverity: minimumSeverity})
	if err != nil {
		printCommandError(stderr, colorMode, "strider lint", "%v", err)
		return exitError
	}
	baselineConfig.selectedCodes = make(map[string]bool, len(registry.Rules()))
	for _, rule := range registry.Rules() {
		baselineConfig.selectedCodes[rule.Meta().Code] = true
	}
	if knownCodes == nil {
		knownCodes = registry.KnownCodes()
	}
	baselineConfig.knownCodes = knownCodes
	if *listRules {
		return listLintRules(registry, colorMode, stdout)
	}
	if *explain != "" {
		return explainLintRule(registry, *explain, colorMode, stdout, stderr)
	}
	if *format != "text" && *format != "json" && *format != "html" {
		printCommandError(stderr, colorMode, "strider lint", "unsupported report format %q", *format)
		return exitError
	}
	return lintPaths(flags.Args(), *format, registry, configuration.Root, lintConfig.Excludes, baselineConfig, colorMode, stdout, stderr)
}

func listLintRules(registry *syntax.Registry, colorMode ui.ColorMode, stdout io.Writer) int {
	palette := ui.NewPalette(stdout, colorMode)
	entries := make([]ruleListEntry, 0, len(registry.Rules()))
	for _, rule := range registry.Rules() {
		meta := rule.Meta()
		entries = append(entries, ruleListEntry{code: meta.Code, severity: registry.Severity(meta.Code), summary: meta.Summary})
	}
	writeRuleList(stdout, palette, entries)
	return exitSuccess
}

func explainLintRule(registry *syntax.Registry, code string, colorMode ui.ColorMode, stdout, stderr io.Writer) int {
	palette := ui.NewPalette(stdout, colorMode)
	for _, rule := range registry.Rules() {
		meta := rule.Meta()
		if meta.Code != code {
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
	printCommandError(stderr, colorMode, "strider lint", "unknown lint rule %q", code)
	return exitError
}

func lintPaths(
	paths []string,
	format string,
	registry *syntax.Registry,
	root string,
	excludes []string,
	baselineConfig baselineOptions,
	colorMode ui.ColorMode,
	stdout,
	stderr io.Writer,
) int {
	files, err := source.Discover(paths, source.Options{SkipGenerated: true})
	if err != nil {
		printCommandError(stderr, colorMode, "strider lint", "%v", err)
		return exitError
	}
	files = filterFiles(files, root, excludes)
	diagnostics, err := syntax.Run(files, registry)
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
		err = syntax.ReportJSON(stdout, diagnostics)
	} else if format == "html" {
		err = syntax.ReportHTML(stdout, diagnostics)
	} else {
		err = syntax.ReportText(stdout, diagnostics, colorMode)
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

func runAnalyze(args []string, configuration config.Config, colorMode ui.ColorMode, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("analyze", flag.ContinueOnError)
	flags.SetOutput(stderr)
	aliases := checkCommandAliases()
	format := stringOption(flags, "format", "f", "text", "report format: text, json, or html")
	minimumSeverityFlag := stringOption(flags, "minimum-severity", "s", "", "minimum effective severity: none, note, warning, or error")
	listRules := boolOption(flags, "list-rules", "l", false, "list analysis rules admitted by the severity floor")
	explain := stringOption(flags, "explain", "e", "", "explain an analysis rule")
	baselinePath := stringOption(flags, "baseline", "b", "", "path to the analysis baseline")
	generateBaseline := boolOption(flags, "generate-baseline", "g", false, "replace the baseline with all current findings")
	removeOutdated := boolOption(flags, "remove-outdated-baseline-entries", "r", false, "remove baseline entries that no longer match")
	var only stringList
	varOption(flags, &only, "only", "o", "run only these rule codes (repeatable or comma-separated)")
	flags.Usage = func() {
		palette := ui.NewPalette(stderr, colorMode)
		fmt.Fprintln(stderr, palette.Accent("Usage:")+" strider analyze [OPTIONS] [FILE|DIR]...")
		printFlagDefaults(stderr, flags, aliases, palette)
	}
	if !parseCommandFlags(flags, args, aliases, "analyze", colorMode, stderr) {
		return exitError
	}
	analyzeConfig := configuration.Checks
	semanticSettings, knownCodes, settingsErr := commandSettings(analyzeConfig.Rules, checkengine.CapabilityAST)
	if settingsErr != nil {
		printCommandError(stderr, colorMode, "strider analyze", "%v", settingsErr)
		return exitError
	}
	minimumSeverity, ok := resolveMinimumSeverity(flags, *minimumSeverityFlag, analyzeConfig.MinimumSeverity, "analyze", colorMode, stderr)
	if !ok {
		return exitError
	}
	baselineConfig, ok := resolveBaselineOptions(flags, configuration, analyzeConfig, *baselinePath, *generateBaseline, *removeOutdated, stderr, "analyze", colorMode)
	if !ok {
		return exitError
	}
	registry, err := semantic.NewRegistryWithOptions(semantic.RegistryOptions{Only: only, Settings: semanticSettings, Root: configuration.Root, MinimumSeverity: minimumSeverity})
	if err != nil {
		printCommandError(stderr, colorMode, "strider analyze", "%v", err)
		return exitError
	}
	baselineConfig.selectedCodes = make(map[string]bool, len(registry.Rules()))
	for _, rule := range registry.Rules() {
		baselineConfig.selectedCodes[rule.Meta().Code] = true
	}
	if knownCodes == nil {
		knownCodes = registry.KnownCodes()
	}
	baselineConfig.knownCodes = knownCodes
	if *listRules {
		return listAnalyzeRules(registry, colorMode, stdout)
	}
	if *explain != "" {
		return explainAnalyzeRule(registry, *explain, colorMode, stdout, stderr)
	}
	if *format != "text" && *format != "json" && *format != "html" {
		printCommandError(stderr, colorMode, "strider analyze", "unsupported report format %q", *format)
		return exitError
	}
	return analyzePaths(flags.Args(), *format, registry, configuration.Root, analyzeConfig.Excludes, baselineConfig, colorMode, stdout, stderr)
}

func listAnalyzeRules(registry *semantic.Registry, colorMode ui.ColorMode, stdout io.Writer) int {
	palette := ui.NewPalette(stdout, colorMode)
	entries := make([]ruleListEntry, 0, len(registry.Rules()))
	for _, rule := range registry.Rules() {
		meta := rule.Meta()
		entries = append(entries, ruleListEntry{code: meta.Code, severity: registry.Severity(meta.Code), summary: meta.Summary})
	}
	writeRuleList(stdout, palette, entries)
	return exitSuccess
}

func explainAnalyzeRule(registry *semantic.Registry, code string, colorMode ui.ColorMode, stdout, stderr io.Writer) int {
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
	printCommandError(stderr, colorMode, "strider analyze", "unknown analysis rule %q", code)
	return exitError
}

func analyzePaths(
	paths []string,
	format string,
	registry *semantic.Registry,
	root string,
	excludes []string,
	baselineConfig baselineOptions,
	colorMode ui.ColorMode,
	stdout,
	stderr io.Writer,
) int {
	diagnostics, err := semantic.Run(paths, registry)
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
		err = semantic.ReportJSON(stdout, diagnostics)
	} else if format == "html" {
		err = semantic.ReportHTML(stdout, diagnostics)
	} else {
		err = semantic.ReportText(stdout, diagnostics, colorMode)
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
	return colorSeverityText(severity, string(severity), palette)
}

func colorSeverityText(severity diagnostic.Severity, text string, palette ui.Palette) string {
	switch severity {
	case diagnostic.SeverityError:
		return palette.Error(text)
	case diagnostic.SeverityNote:
		return palette.Note(text)
	case diagnostic.SeverityNone:
		return palette.Muted(text)
	default:
		return palette.Warning(text)
	}
}

type ruleListEntry struct {
	code     string
	severity diagnostic.Severity
	summary  string
}

func writeRuleList(writer io.Writer, palette ui.Palette, entries []ruleListEntry) {
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].code < entries[right].code
	})
	codeWidth := 0
	for _, entry := range entries {
		codeWidth = max(codeWidth, len(entry.code))
	}
	for _, entry := range entries {
		code := fmt.Sprintf("%-*s", codeWidth, entry.code)
		severity := fmt.Sprintf("%-7s", entry.severity)
		fmt.Fprintf(writer, "%s  %s  %s\n", colorSeverityText(entry.severity, code, palette), colorSeverityText(entry.severity, severity, palette), entry.summary)
	}
}

func resolveMinimumSeverity(flags *flag.FlagSet, flagValue, configured string, command string, colorMode ui.ColorMode, stderr io.Writer) (diagnostic.Severity, bool) {
	value := configured
	if flagWasSetAny(flags, "minimum-severity", "s") {
		value = flagValue
	}
	severity := diagnostic.Severity(value)
	if diagnostic.ValidSeverity(severity) {
		return severity, true
	}
	printCommandError(stderr, colorMode, "strider "+command, "--minimum-severity must be none, note, warning, or error")
	return "", false
}

func commandSettings(settings map[string]config.RuleConfig, capability checkengine.Capability) (map[string]config.RuleConfig, map[string]bool, error) {
	registry, err := checkengine.NewRegistry(checkengine.RegistryOptions{Settings: settings, MinimumSeverity: diagnostic.SeverityNone})
	if err != nil {
		return nil, nil, err
	}
	normalized := make(map[string]config.RuleConfig, len(settings))
	for code, setting := range settings {
		normalized[strings.ToLower(code)] = setting
	}
	filtered := make(map[string]config.RuleConfig)
	for _, rule := range registry.Rules() {
		meta := rule.Meta()
		setting, configured := normalized[strings.ToLower(meta.Code)]
		if !configured {
			continue
		}
		if meta.Code != "format" && meta.Capabilities&capability != 0 {
			filtered[meta.Code] = setting
		}
	}
	return filtered, registry.KnownCodes(), nil
}

func resolveBaselineOptions(
	flags *flag.FlagSet,
	configuration config.Config,
	tool config.ToolConfig,
	path string,
	generate,
	prune bool,
	stderr io.Writer,
	command string,
	colorMode ui.ColorMode,
) (baselineOptions, bool) {
	if !flagWasSetAny(flags, "baseline", "b") {
		path = configuration.Resolve(tool.Baseline)
	}
	if generate && prune {
		printCommandError(stderr, colorMode, "strider "+command, "--generate-baseline and --remove-outdated-baseline-entries are mutually exclusive")
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
	return baselineOptions{path: path, generate: generate, prune: prune}, true
}

func applyBaseline(command string, diagnostics []diagnostic.Diagnostic, options baselineOptions, colorMode ui.ColorMode, stderr io.Writer) ([]diagnostic.Diagnostic, bool, error) {
	if options.path == "" {
		return diagnostics, false, nil
	}
	if options.generate {
		generated, err := baseline.Generate(options.path, diagnostics)
		if err != nil {
			return nil, false, err
		}
		if err := baseline.Write(options.path, generated); err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}
	loaded, err := baseline.Load(options.path)
	if err != nil {
		return nil, false, fmt.Errorf("load baseline %s: %w", options.path, err)
	}
	result, err := baseline.ApplyCatalogSelection(options.path, loaded, diagnostics, options.selectedCodes, options.knownCodes)
	if err != nil {
		return nil, false, err
	}
	if options.prune {
		if err := baseline.Write(options.path, result.Matched); err != nil {
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

func filterDiagnostics(diagnostics []diagnostic.Diagnostic, root string, excludes []string) []diagnostic.Diagnostic {
	filtered := make([]diagnostic.Diagnostic, 0, len(diagnostics))
	for _, item := range diagnostics {
		if !pathfilter.Matches(root, item.File, excludes) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
