//strider:ignore-file cognitive-complexity,confusing-results,cyclomatic-complexity,function-length,import-shadowing,single-case-switch
package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/source"
	"github.com/gempir/strider/internal/telemetry"
	"github.com/gempir/strider/internal/ui"
	"github.com/gempir/strider/internal/workspace"
)

const (
	diffEqual  = ' '
	diffRemove = '-'
	diffAdd    = '+'
)

type formatOptions struct {
	check         bool
	diff          bool
	write         bool
	stdin         bool
	stdinFilename string
	paths         []string
	formatter     formatter.Options
	root          string
	directory     string
	excludes      []string
	colorMode     ui.ColorMode
}

type sourceLine struct {
	text     string
	newline  bool
	carriage bool
}

type diffOperation struct {
	kind byte
	line sourceLine
}

type diffHunk struct {
	start int
	end   int
}

func runFormat(ctx context.Context, args []string, configuration config.Config, colorMode ui.ColorMode, stdin io.Reader, stdout, stderr io.Writer) int {
	if err := ctx.Err(); err != nil {
		printCommandError(stderr, colorMode, "strider fmt", "%v", err)
		return exitError
	}
	options, ok := parseFormatOptions(args, colorMode, stderr)
	if !ok {
		return exitError
	}
	options.formatter = formatter.Options{
		PrintWidth: configuration.Formatter.PrintWidth,
	}
	options.root = configuration.Root
	options.directory = configuration.Directory
	options.excludes = configuration.Formatter.Excludes
	options.colorMode = colorMode
	if options.stdin {
		return formatStdin(ctx, options.stdinFilename, options.formatter, colorMode, stdin, stdout, stderr)
	}
	return formatPaths(ctx, options, stdout, stderr)
}

func parseFormatOptions(args []string, colorMode ui.ColorMode, stderr io.Writer) (formatOptions, bool) {
	flags := flag.NewFlagSet("fmt", flag.ContinueOnError)
	flags.SetOutput(stderr)
	aliases := commandOptionAliases["fmt"]
	check := boolOption(flags, "check", "c", "report files that would change without writing")
	diffMode := boolOption(flags, "diff", "d", "print full unified diffs without writing")
	write := boolOption(flags, "write", "w", "write formatted source in place")
	stdinMode := boolOption(flags, "stdin", "s", "read source from stdin and write it to stdout")
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
		paths = []string{
			".",
		}
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

func formatStdin(ctx context.Context, filename string, formatOptions formatter.Options, colorMode ui.ColorMode, stdin io.Reader, stdout, stderr io.Writer) int {
	if err := ctx.Err(); err != nil {
		printCommandError(stderr, colorMode, "strider fmt", "%v", err)
		return exitError
	}
	input, err := io.ReadAll(stdin)
	if err != nil {
		printCommandError(stderr, colorMode, "strider fmt", "%v", err)
		return exitError
	}
	if err := ctx.Err(); err != nil {
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

func formatPaths(ctx context.Context, options formatOptions, stdout, stderr io.Writer) int {
	finish := telemetry.Start("format.total")
	defer finish()
	if err := ctx.Err(); err != nil {
		printCommandError(stderr, options.colorMode, "strider fmt", "%v", err)
		return exitError
	}
	shared, err := workspace.Open(options.paths, workspace.Options{
		SkipGenerated: true,
		Directory:     options.directory,
		Root:          options.root,
		Excludes:      options.excludes,
	})
	if err != nil {
		printCommandError(stderr, options.colorMode, "strider fmt", "%v", err)
		return exitError
	}
	defer shared.Close()
	if err := ctx.Err(); err != nil {
		printCommandError(stderr, options.colorMode, "strider fmt", "%v", err)
		return exitError
	}
	if options.check {
		statuses, formatErrors := formatFileStatuses(ctx, shared.Files(), options.formatter)
		for _, formatErr := range formatErrors {
			if formatErr != nil {
				printCommandError(stderr, options.colorMode, "strider fmt", "%v", formatErr)
				return exitError
			}
		}
		if err := ctx.Err(); err != nil {
			printCommandError(stderr, options.colorMode, "strider fmt", "%v", err)
			return exitError
		}
		if reportFormatStatuses(statuses, options, stdout) {
			return exitFindings
		}
		return exitSuccess
	}
	formatted, formatErrors := formatFiles(ctx, shared.Files(), options.formatter)
	for _, formatErr := range formatErrors {
		if formatErr != nil {
			printCommandError(stderr, options.colorMode, "strider fmt", "%v", formatErr)
			return exitError
		}
	}
	if err := ctx.Err(); err != nil {
		printCommandError(stderr, options.colorMode, "strider fmt", "%v", err)
		return exitError
	}
	reportFinish := telemetry.Start("format.report")
	changed := reportFormatChanges(formatted, options, stdout)
	reportFinish()
	if options.write {
		if err := writeFormattedFiles(formatted); err != nil {
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

func reportFormatStatuses(files []formattedStatus, options formatOptions, stdout io.Writer) bool {
	palette := ui.NewPalette(stdout, options.colorMode)
	changed := false
	for _, file := range files {
		if !file.changed || file.ignored {
			continue
		}
		changed = true
		fmt.Fprintf(stdout, "%s %s\n", palette.Warning("would reformat"), palette.Path(source.DisplayPath(file.filename)))
	}
	return changed
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

func printDiff(writer io.Writer, filename string, before, after []byte, palette ui.Palette) {
	operations := lineDiff(splitSourceLines(before), splitSourceLines(after))
	fmt.Fprintln(writer, palette.Removed("--- "+filename))
	fmt.Fprintln(writer, palette.Added("+++ "+filename))
	for _, hunk := range diffHunks(operations, 3) {
		oldStart, newStart := diffLineNumbers(operations[:hunk.start])
		oldCount, newCount := diffLineCounts(operations[hunk.start:hunk.end])
		if oldCount == 0 {
			oldStart--
		}
		if newCount == 0 {
			newStart--
		}
		fmt.Fprintln(writer, palette.Hunk(fmt.Sprintf("@@ -%d,%d +%d,%d @@", oldStart, oldCount, newStart, newCount)))
		for _, operation := range operations[hunk.start:hunk.end] {
			line := string(operation.kind) + operation.line.text
			switch operation.kind {
			case diffRemove:
				fmt.Fprintln(writer, palette.Removed(line))
			case diffAdd:
				fmt.Fprintln(writer, palette.Added(line))
			default:
				fmt.Fprintln(writer, line)
			}
			if !operation.line.newline {
				fmt.Fprintln(writer, `\ No newline at end of file`)
			}
		}
	}
}

func splitSourceLines(content []byte) []sourceLine {
	if len(content) == 0 {
		return nil
	}
	parts := strings.SplitAfter(string(content), "\n")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	lines := make([]sourceLine, 0, len(parts))
	for _, part := range parts {
		newline := strings.HasSuffix(part, "\n")
		part = strings.TrimSuffix(part, "\n")
		carriage := strings.HasSuffix(part, "\r")
		lines = append(lines, sourceLine{
			text:     strings.TrimSuffix(part, "\r"),
			newline:  newline,
			carriage: carriage,
		})
	}
	return lines
}

func lineDiff(before, after []sourceLine) []diffOperation {
	maximum := len(before) + len(after)
	offset := maximum + 1
	frontier := make([]int, maximum*2+3)
	frontier[offset+1] = 0
	history := make([][]int, 0, maximum+1)
	distance := 0
	found := false
	for distance = 0; distance <= maximum; distance++ {
		for diagonal := -distance; diagonal <= distance; diagonal += 2 {
			index := offset + diagonal
			var oldIndex int
			if diagonal == -distance || diagonal != distance && frontier[index-1] < frontier[index+1] {
				oldIndex = frontier[index+1]
			} else {
				oldIndex = frontier[index-1] + 1
			}
			newIndex := oldIndex - diagonal
			for oldIndex < len(before) && newIndex < len(after) && before[oldIndex] == after[newIndex] {
				oldIndex++
				newIndex++
			}
			frontier[index] = oldIndex
			if oldIndex >= len(before) && newIndex >= len(after) {
				found = true
				break
			}
		}
		history = append(history, append([]int(nil), frontier...))
		if found {
			break
		}
	}

	oldIndex := len(before)
	newIndex := len(after)
	reversed := make([]diffOperation, 0, oldIndex+newIndex)
	for current := distance; current > 0; current-- {
		previous := history[current-1]
		diagonal := oldIndex - newIndex
		previousDiagonal := diagonal - 1
		if diagonal == -current || diagonal != current && previous[offset+diagonal-1] < previous[offset+diagonal+1] {
			previousDiagonal = diagonal + 1
		}
		previousOld := previous[offset+previousDiagonal]
		previousNew := previousOld - previousDiagonal
		for oldIndex > previousOld && newIndex > previousNew {
			oldIndex--
			newIndex--
			reversed = append(reversed, diffOperation{
				kind: diffEqual,
				line: before[oldIndex],
			})
		}
		if oldIndex == previousOld {
			newIndex--
			reversed = append(reversed, diffOperation{
				kind: diffAdd,
				line: after[newIndex],
			})
		} else {
			oldIndex--
			reversed = append(reversed, diffOperation{
				kind: diffRemove,
				line: before[oldIndex],
			})
		}
	}
	for oldIndex > 0 && newIndex > 0 {
		oldIndex--
		newIndex--
		reversed = append(reversed, diffOperation{
			kind: diffEqual,
			line: before[oldIndex],
		})
	}
	for oldIndex > 0 {
		oldIndex--
		reversed = append(reversed, diffOperation{
			kind: diffRemove,
			line: before[oldIndex],
		})
	}
	for newIndex > 0 {
		newIndex--
		reversed = append(reversed, diffOperation{
			kind: diffAdd,
			line: after[newIndex],
		})
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return reversed
}

func diffHunks(operations []diffOperation, context int) []diffHunk {
	hunks := make([]diffHunk, 0, len(operations))
	for index, operation := range operations {
		if operation.kind == diffEqual {
			continue
		}
		candidate := diffHunk{
			start: max(0, index-context),
			end:   min(len(operations), index+context+1),
		}
		if len(hunks) != 0 && candidate.start <= hunks[len(hunks)-1].end {
			hunks[len(hunks)-1].end = max(hunks[len(hunks)-1].end, candidate.end)
			continue
		}
		hunks = append(hunks, candidate)
	}
	return hunks
}

func diffLineNumbers(operations []diffOperation) (int, int) {
	oldLine := 1
	newLine := 1
	for _, operation := range operations {
		if operation.kind != diffAdd {
			oldLine++
		}
		if operation.kind != diffRemove {
			newLine++
		}
	}
	return oldLine, newLine
}

func diffLineCounts(operations []diffOperation) (int, int) {
	oldCount := 0
	newCount := 0
	for _, operation := range operations {
		if operation.kind != diffAdd {
			oldCount++
		}
		if operation.kind != diffRemove {
			newCount++
		}
	}
	return oldCount, newCount
}
