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
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/lint"
	"github.com/gempir/strider/internal/source"
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
}

const (
	exitSuccess  = 0
	exitFindings = 1
	exitError    = 2
)

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitError
	}
	switch args[0] {
	case "fmt", "format":
		return runFormat(args[1:], stdin, stdout, stderr)
	case "lint":
		return runLint(args[1:], stdout, stderr)
	case "analyze":
		return runAnalyze(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		usage(stdout)
		return exitSuccess
	case "version", "--version":
		fmt.Fprintln(stdout, "strider dev")
		return exitSuccess
	default:
		fmt.Fprintf(stderr, "strider: unknown command %q\n\n", args[0])
		usage(stderr)
		return exitError
	}
}

func usage(writer io.Writer) {
	fmt.Fprintln(
		writer,
		`Strider formats, lints, and statically analyzes Go code.

Usage:
  strider fmt [--check|--diff|--write|--stdin] [FILE|DIR]...
  strider fmt --stdin                      # stdin to stdout
  strider lint [OPTIONS] [FILE|DIR]...
  strider analyze [OPTIONS] [FILE|DIR]...

Commands:
  fmt       Format Go source (alias: format)
  lint      Run clarity and safety rules
  analyze   Run package-aware static analysis
  version   Print the version`,
	)
}

func runFormat(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	options, ok := parseFormatOptions(args, stderr)
	if !ok {
		return exitError
	}
	if options.stdin {
		return formatStdin(options.stdinFilename, stdin, stdout, stderr)
	}
	return formatPaths(options, stdout, stderr)
}

func parseFormatOptions(args []string, stderr io.Writer) (formatOptions, bool) {
	flags := flag.NewFlagSet("fmt", flag.ContinueOnError)
	flags.SetOutput(stderr)
	check := flags.Bool("check", false, "report files that would change without writing")
	diffMode := flags.Bool("diff", false, "print full unified diffs without writing")
	write := flags.Bool("write", false, "write formatted source in place")
	stdinMode := flags.Bool("stdin", false, "read source from stdin and write it to stdout")
	stdinFilename := flags.String("stdin-filename", "<stdin>", "logical filename for stdin")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: strider fmt [--check|--diff|--write|--stdin] [FILE|DIR]...")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return formatOptions{}, false
	}
	stdinFilenameSet := flagWasSet(flags, "stdin-filename")
	if stdinFilenameSet && !*stdinMode {
		fmt.Fprintln(stderr, "strider fmt: --stdin-filename requires --stdin")
		return formatOptions{}, false
	}
	modeCount := boolInt(*check) + boolInt(*diffMode) + boolInt(*write)
	if modeCount > 1 {
		fmt.Fprintln(stderr, "strider fmt: --check, --diff, and --write are mutually exclusive")
		return formatOptions{}, false
	}
	paths := flags.Args()
	if *stdinMode {
		if len(paths) != 0 {
			fmt.Fprintln(stderr, "strider fmt: --stdin does not accept file or directory arguments")
			return formatOptions{}, false
		}
		if modeCount != 0 {
			fmt.Fprintln(
				stderr,
				"strider fmt: formatting stdin does not accept --check, --diff, or --write",
			)
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

func formatStdin(filename string, stdin io.Reader, stdout, stderr io.Writer) int {
	input, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "strider fmt: %v\n", err)
		return exitError
	}
	result, err := formatter.Format(filename, input)
	if err != nil {
		fmt.Fprintf(stderr, "strider fmt: %v\n", err)
		return exitError
	}
	if _, err := stdout.Write(result.Source); err != nil {
		fmt.Fprintf(stderr, "strider fmt: %v\n", err)
		return exitError
	}
	return exitSuccess
}

func formatPaths(options formatOptions, stdout, stderr io.Writer) int {
	files, err := source.Discover(options.paths, source.Options{SkipGenerated: true})
	if err != nil {
		fmt.Fprintf(stderr, "strider fmt: %v\n", err)
		return exitError
	}
	formatted := make([]formattedFile, 0, len(files))
	for _, filename := range files {
		original, readErr := os.ReadFile(filename)
		if readErr != nil {
			fmt.Fprintf(stderr, "strider fmt: %s: %v\n", source.DisplayPath(filename), readErr)
			return exitError
		}
		result, formatErr := formatter.Format(filename, original)
		if formatErr != nil {
			fmt.Fprintf(stderr, "strider fmt: %v\n", formatErr)
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
			fmt.Fprintf(stderr, "strider fmt: %v\n", err)
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
	changed := false
	for _, file := range files {
		if !file.result.Changed {
			continue
		}
		changed = true
		switch {
		case options.check:
			fmt.Fprintln(stdout, source.DisplayPath(file.filename))
		case options.diff:
			printDiff(stdout, source.DisplayPath(file.filename), file.original, file.result.Source)
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

func printDiff(writer io.Writer, filename string, before, after []byte) {
	beforeLines := splitLines(before)
	afterLines := splitLines(after)
	fmt.Fprintf(
		writer,
		"--- %s\n+++ %s\n@@ -1,%d +1,%d @@\n",
		filename,
		filename,
		len(beforeLines),
		len(afterLines),
	)
	for _, line := range beforeLines {
		fmt.Fprintf(writer, "-%s\n", line)
	}
	for _, line := range afterLines {
		fmt.Fprintf(writer, "+%s\n", line)
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

func runLint(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("lint", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "text", "report format: text or json")
	listRules := flags.Bool("list-rules", false, "list enabled lint rules")
	allRules := flags.Bool("all-rules", false, "run every built-in rule")
	explain := flags.String("explain", "", "explain a lint rule")
	var only stringList
	flags.Var(&only, "only", "run only these rule codes (repeatable or comma-separated)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: strider lint [OPTIONS] [FILE|DIR]...")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitError
	}
	if *allRules && len(only) != 0 {
		fmt.Fprintln(stderr, "strider lint: --all-rules and --only are mutually exclusive")
		return exitError
	}
	var registry *lint.Registry
	var err error
	if *allRules {
		registry, err = lint.NewRegistryAll()
	} else {
		registry, err = lint.NewRegistry(only)
	}
	if err != nil {
		fmt.Fprintf(stderr, "strider lint: %v\n", err)
		return exitError
	}
	if *listRules {
		return listLintRules(registry, stdout)
	}
	if *explain != "" {
		return explainLintRule(registry, *explain, stdout, stderr)
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "strider lint: unsupported report format %q\n", *format)
		return exitError
	}
	return lintPaths(flags.Args(), *format, registry, stdout, stderr)
}

func listLintRules(registry *lint.Registry, stdout io.Writer) int {
	rules := registry.Rules()
	sort.Slice(
		rules,
		func(i, j int) bool {
			return rules[i].Meta().Code < rules[j].Meta().Code
		},
	)
	for _, rule := range rules {
		meta := rule.Meta()
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", meta.Code, meta.DefaultSeverity, meta.Summary)
	}
	return exitSuccess
}

func explainLintRule(registry *lint.Registry, code string, stdout, stderr io.Writer) int {
	for _, rule := range registry.Rules() {
		meta := rule.Meta()
		if meta.Code != code {
			continue
		}
		fmt.Fprintf(
			stdout,
			"%s (%s)\n\n%s\n\nGood:\n%s\n\nBad:\n%s\n",
			meta.Code,
			meta.DefaultSeverity,
			meta.Explanation,
			meta.GoodExample,
			meta.BadExample,
		)
		return exitSuccess
	}
	fmt.Fprintf(stderr, "strider lint: unknown lint rule %q\n", code)
	return exitError
}

func lintPaths(paths []string, format string, registry *lint.Registry, stdout, stderr io.Writer) int {
	files, err := source.Discover(paths, source.Options{SkipGenerated: true})
	if err != nil {
		fmt.Fprintf(stderr, "strider lint: %v\n", err)
		return exitError
	}
	diagnostics, err := lint.Run(files, registry)
	if err != nil {
		fmt.Fprintf(stderr, "strider lint: %v\n", err)
		return exitError
	}
	if format == "json" {
		err = lint.ReportJSON(stdout, diagnostics)
	} else {
		err = lint.ReportText(stdout, diagnostics)
	}
	if err != nil {
		fmt.Fprintf(stderr, "strider lint: %v\n", err)
		return exitError
	}
	if len(diagnostics) != 0 {
		return exitFindings
	}
	return exitSuccess
}

func runAnalyze(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("analyze", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "text", "report format: text or json")
	listRules := flags.Bool("list-rules", false, "list enabled analysis rules")
	explain := flags.String("explain", "", "explain an analysis rule")
	var only stringList
	flags.Var(&only, "only", "run only these rule codes (repeatable or comma-separated)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: strider analyze [OPTIONS] [FILE|DIR]...")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitError
	}
	registry, err := analyze.NewRegistry(only)
	if err != nil {
		fmt.Fprintf(stderr, "strider analyze: %v\n", err)
		return exitError
	}
	if *listRules {
		return listAnalyzeRules(registry, stdout)
	}
	if *explain != "" {
		return explainAnalyzeRule(registry, *explain, stdout, stderr)
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "strider analyze: unsupported report format %q\n", *format)
		return exitError
	}
	return analyzePaths(flags.Args(), *format, registry, stdout, stderr)
}

func listAnalyzeRules(registry *analyze.Registry, stdout io.Writer) int {
	rules := registry.Rules()
	sort.Slice(
		rules,
		func(i, j int) bool {
			return rules[i].Meta().Code < rules[j].Meta().Code
		},
	)
	for _, rule := range rules {
		meta := rule.Meta()
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", meta.Code, meta.DefaultSeverity, meta.Summary)
	}
	return exitSuccess
}

func explainAnalyzeRule(
	registry *analyze.Registry,
	code string,
	stdout, stderr io.Writer,
) int {
	for _, rule := range registry.Rules() {
		meta := rule.Meta()
		if !strings.EqualFold(meta.Code, code) {
			continue
		}
		fmt.Fprintf(
			stdout,
			"%s (%s)\n\n%s\n\nGood:\n%s\n\nBad:\n%s\n",
			meta.Code,
			meta.DefaultSeverity,
			meta.Explanation,
			meta.GoodExample,
			meta.BadExample,
		)
		return exitSuccess
	}
	fmt.Fprintf(stderr, "strider analyze: unknown analysis rule %q\n", code)
	return exitError
}

func analyzePaths(
	paths []string,
	format string,
	registry *analyze.Registry,
	stdout, stderr io.Writer,
) int {
	diagnostics, err := analyze.Run(paths, registry)
	if err != nil {
		fmt.Fprintf(stderr, "strider analyze: %v\n", err)
		return exitError
	}
	if format == "json" {
		err = analyze.ReportJSON(stdout, diagnostics)
	} else {
		err = analyze.ReportText(stdout, diagnostics)
	}
	if err != nil {
		fmt.Fprintf(stderr, "strider analyze: %v\n", err)
		return exitError
	}
	if len(diagnostics) != 0 {
		return exitFindings
	}
	return exitSuccess
}
