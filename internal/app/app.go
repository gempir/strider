// Package app implements the Strider command-line application.
package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/gempir/strider/internal/baseline"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/report"
	"github.com/gempir/strider/internal/ui"
)

const (
	exitSuccess  = 0
	exitFindings = 1
	exitError    = 2
)

var version = "dev"

type baselineOptions struct {
	path          string
	root          string
	generate      bool
	prune         bool
	selectedCodes map[string]bool
	knownCodes    map[string]bool
}

type checkListEntry struct {
	code     string
	severity diagnostic.Severity
	summary  string
}

func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	directory, err := os.Getwd()
	if err != nil {
		printError(stderr, ui.ColorAuto, "strider", err)
		return exitError
	}
	return runFrom(ctx, directory, args, stdin, stdout, stderr)
}

func runFrom(ctx context.Context, directory string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		printError(stderr, ui.ColorAuto, "strider", err)
		return exitError
	}
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
		configuration, err := config.Load(directory, globals.configPath, globals.noConfig)
		if err != nil {
			printError(stderr, colorMode, "strider", err)
			return exitError
		}
		colorMode = configuredColor(configuration, globals)
		return runCheck(ctx, args[1:], configuration, colorMode, stdout, stderr)
	case "fmt", "format":
		configuration, err := config.Load(directory, globals.configPath, globals.noConfig)
		if err != nil {
			printError(stderr, colorMode, "strider", err)
			return exitError
		}
		colorMode = configuredColor(configuration, globals)
		return runFormat(ctx, args[1:], configuration, colorMode, stdin, stdout, stderr)
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

func colorSeverity(severity diagnostic.Severity, palette ui.Palette) string {
	return report.StyleSeverity(severity, string(severity), palette)
}

func writeCheckList(writer io.Writer, palette ui.Palette, entries []checkListEntry) {
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
		fmt.Fprintf(writer, "%s  %s  %s\n", report.StyleSeverity(entry.severity, code, palette), report.StyleSeverity(entry.severity, severity, palette), entry.summary)
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
	return baselineOptions{
		path:     path,
		root:     configuration.Root,
		generate: generate,
		prune:    prune,
	}, true
}

func applyBaseline(command string, diagnostics []diagnostic.Diagnostic, options baselineOptions, colorMode ui.ColorMode, stderr io.Writer) ([]diagnostic.Diagnostic, bool, error) {
	if options.path == "" {
		return diagnostics, false, nil
	}
	if options.generate {
		generated, err := baseline.Generate(options.path, options.root, diagnostics)
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
	result, err := baseline.Apply(options.path, options.root, loaded, diagnostics, options.selectedCodes, options.knownCodes)
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
