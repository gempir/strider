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

	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/lint"
	"github.com/gempir/strider/internal/source"
)

type formattedFile struct {
	filename string
	original []byte
	result   formatter.Result
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
	fmt.Fprintln(writer, `Strider is a strict formatter and syntax linter for Go.

Usage:
  strider fmt [--check|--diff|--write|--stdin] [FILE|DIR]...
  strider fmt --stdin                      # stdin to stdout
  strider lint [OPTIONS] [FILE|DIR]...

Commands:
  fmt       Format Go source (alias: format)
  lint      Run clarity and safety rules
  version   Print the version`)
}

func runFormat(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
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
		return exitError
	}
	stdinFilenameSet := false
	flags.Visit(func(current *flag.Flag) {
		if current.Name == "stdin-filename" {
			stdinFilenameSet = true
		}
	})
	if stdinFilenameSet && !*stdinMode {
		fmt.Fprintln(stderr, "strider fmt: --stdin-filename requires --stdin")
		return exitError
	}
	modeCount := boolInt(*check) + boolInt(*diffMode) + boolInt(*write)
	if modeCount > 1 {
		fmt.Fprintln(stderr, "strider fmt: --check, --diff, and --write are mutually exclusive")
		return exitError
	}

	paths := flags.Args()
	if *stdinMode {
		if len(paths) != 0 {
			fmt.Fprintln(stderr, "strider fmt: --stdin does not accept file or directory arguments")
			return exitError
		}
		if modeCount != 0 {
			fmt.Fprintln(stderr, "strider fmt: formatting stdin does not accept --check, --diff, or --write")
			return exitError
		}
		input, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "strider fmt: %v\n", err)
			return exitError
		}
		result, err := formatter.Format(*stdinFilename, input)
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
	if len(paths) == 0 {
		paths = []string{"."}
	}
	if modeCount == 0 {
		*write = true
	}

	files, err := source.Discover(paths, source.Options{SkipGenerated: true})
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
		formatted = append(formatted, formattedFile{filename: filename, original: original, result: result})
	}

	changed := false
	for _, file := range formatted {
		if !file.result.Changed {
			continue
		}
		changed = true
		switch {
		case *check:
			fmt.Fprintln(stdout, source.DisplayPath(file.filename))
		case *diffMode:
			printDiff(stdout, source.DisplayPath(file.filename), file.original, file.result.Source)
		}
	}
	if *write {
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
		info, err := os.Stat(file.filename)
		if err != nil {
			cleanup()
			return err
		}
		temporary, err := os.CreateTemp(filepath.Dir(file.filename), ".strider-*")
		if err != nil {
			cleanup()
			return err
		}
		name := temporary.Name()
		if err := temporary.Chmod(info.Mode().Perm()); err == nil {
			_, err = temporary.Write(file.result.Source)
		}
		if closeErr := temporary.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(name)
			cleanup()
			return err
		}
		staged = append(staged, stagedFile{temporary: name, target: file.filename})
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

func printDiff(writer io.Writer, filename string, before, after []byte) {
	beforeLines := splitLines(before)
	afterLines := splitLines(after)
	fmt.Fprintf(writer, "--- %s\n+++ %s\n@@ -1,%d +1,%d @@\n", filename, filename, len(beforeLines), len(afterLines))
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

func (values *stringList) String() string { return strings.Join(*values, ",") }
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
	registry, err := lint.NewRegistry(only)
	if err != nil {
		fmt.Fprintf(stderr, "strider lint: %v\n", err)
		return exitError
	}
	if *listRules {
		rules := registry.Rules()
		sort.Slice(rules, func(i, j int) bool { return rules[i].Meta().Code < rules[j].Meta().Code })
		for _, rule := range rules {
			meta := rule.Meta()
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", meta.Code, meta.DefaultSeverity, meta.Summary)
		}
		return exitSuccess
	}
	if *explain != "" {
		for _, rule := range registry.Rules() {
			meta := rule.Meta()
			if meta.Code != *explain {
				continue
			}
			fmt.Fprintf(stdout, "%s (%s)\n\n%s\n\nGood:\n%s\n\nBad:\n%s\n",
				meta.Code, meta.DefaultSeverity, meta.Explanation, meta.GoodExample, meta.BadExample)
			return exitSuccess
		}
		fmt.Fprintf(stderr, "strider lint: unknown lint rule %q\n", *explain)
		return exitError
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "strider lint: unsupported report format %q\n", *format)
		return exitError
	}

	files, err := source.Discover(flags.Args(), source.Options{SkipGenerated: true})
	if err != nil {
		fmt.Fprintf(stderr, "strider lint: %v\n", err)
		return exitError
	}
	diagnostics, err := lint.Run(files, registry)
	if err != nil {
		fmt.Fprintf(stderr, "strider lint: %v\n", err)
		return exitError
	}
	if *format == "json" {
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
