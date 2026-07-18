// Package report renders shared human-readable diagnostics.
package report

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/ui"
)

// Text writes rich, source-annotated diagnostics and a severity summary.
func Text(writer io.Writer, diagnostics []diagnostic.Diagnostic, colorMode ui.ColorMode) error {
	palette := ui.NewPalette(writer, colorMode)
	sources := make(map[string][]string)
	missing := make(map[string]bool)
	counts := make(map[diagnostic.Severity]int)

	for index, item := range diagnostics {
		counts[item.Severity]++
		if index != 0 {
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}
		if err := writeDiagnostic(writer, item, palette, sources, missing); err != nil {
			return err
		}
	}
	if len(diagnostics) == 0 {
		_, err := fmt.Fprintln(writer, summary(diagnostics, counts, palette))
		return err
	}
	if err := writeCheckCounts(writer, diagnostics, palette); err != nil {
		return err
	}
	_, err := fmt.Fprintln(writer, summary(diagnostics, counts, palette))
	return err
}

type checkCount struct {
	code string
	count int
	severity diagnostic.Severity
}

func writeCheckCounts(writer io.Writer, diagnostics []diagnostic.Diagnostic, palette ui.Palette) error {
	byCode := make(map[string]checkCount)
	codeWidth := 0
	for _, item := range diagnostics {
		entry := byCode[item.Code]
		entry.code = item.Code
		entry.count++
		if item.Severity.AtLeast(entry.severity) || entry.severity == "" {
			entry.severity = item.Severity
		}
		byCode[item.Code] = entry
		codeWidth = max(codeWidth, utf8.RuneCountInString(item.Code))
	}
	entries := make([]checkCount, 0, len(byCode))
	for _, entry := range byCode {
		entries = append(entries, entry)
	}
	sort.Slice(
		entries,
		func(left, right int) bool {
			if entries[left].count != entries[right].count {
				return entries[left].count > entries[right].count
			}
			return entries[left].code < entries[right].code
		},
	)
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	for _, entry := range entries {
		code := fmt.Sprintf("%-*s", codeWidth, entry.code)
		count := styledSeverity(entry.severity, fmt.Sprintf("%d ×", entry.count), palette)
		if _, err := fmt.Fprintf(writer, "%s  %s\n", palette.Code(code), count); err != nil {
			return err
		}
	}
	return nil
}

func writeDiagnostic(
	writer io.Writer,
	item diagnostic.Diagnostic,
	palette ui.Palette,
	sources map[string][]string,
	missing map[string]bool,
) error {
	severity := styledSeverity(item.Severity, string(item.Severity), palette)
	code := palette.Code("[" + item.Code + "]")
	if _, err := fmt.Fprintf(writer, "%s%s: %s\n", severity, code, palette.Bold(item.Message)); err != nil {
		return err
	}
	location := fmt.Sprintf("%s:%d:%d", item.File, item.Start.Line, item.Start.Column)
	if _, err := fmt.Fprintf(writer, "  %s %s\n", palette.Accent("┌─"), palette.Path(location)); err != nil {
		return err
	}

	lines := sourceLines(item.File, sources, missing)
	if item.Start.Line > 0 && item.Start.Line <= len(lines) {
		line := lines[item.Start.Line - 1]
		width := len(strconv.Itoa(item.Start.Line))
		gutter := palette.Accent("│")
		if _, err := fmt.Fprintf(writer, "%*s %s\n", width, "", gutter); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer, "%*d %s %s\n", width, item.Start.Line, gutter, line); err != nil {
			return err
		}
		column := max(item.Start.Column, 1)
		markerWidth := markerWidth(item, line, column)
		marker := styledSeverity(item.Severity, strings.Repeat("^", markerWidth), palette)
		if _, err := fmt.Fprintf(
			writer,
			"%*s %s %s%s\n",
			width,
			"",
			gutter,
			markerIndent(line, column),
			marker,
		); err != nil {
			return err
		}
	}

	for _, note := range item.Notes {
		if _, err := fmt.Fprintf(
			writer,
			"  %s %s: %s\n",
			palette.Accent("="),
			palette.Note("note"),
			note.Message,
		); err != nil {
			return err
		}
	}
	for _, fix := range item.Fixes {
		label := "help"
		style := palette.Success
		if fix.Safety != diagnostic.Safe {
			label = string(fix.Safety) + " fix"
			style = palette.Warning
		}
		if _, err := fmt.Fprintf(
			writer,
			"  %s %s: %s\n",
			palette.Accent("="),
			style(label),
			fix.Message,
		); err != nil {
			return err
		}
	}
	return nil
}

func markerIndent(line string, column int) string {
	prefixLength := min(max(column - 1, 0), len(line))
	var indent strings.Builder
	for _, character := range line[:prefixLength] {
		if character == '\t' {
			indent.WriteByte('\t')
		} else {
			indent.WriteByte(' ')
		}
	}
	return indent.String()
}

func sourceLines(filename string, cache map[string][]string, missing map[string]bool) []string {
	if lines, ok := cache[filename]; ok {
		return lines
	}
	if missing[filename] {
		return nil
	}
	contents, err := os.ReadFile(filename)
	if err != nil {
		missing[filename] = true
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(contents), "\r\n", "\n"), "\n")
	cache[filename] = lines
	return lines
}

func markerWidth(item diagnostic.Diagnostic, line string, column int) int {
	start := min(max(column - 1, 0), len(line))
	remaining := max(utf8.RuneCountInString(line[start:]), 1)
	if item.End.Line == item.Start.Line && item.End.Column > item.Start.Column {
		end := min(max(item.End.Column - 1, start), len(line))
		return min(max(utf8.RuneCountInString(line[start:end]), 1), remaining)
	}
	return remaining
}

func styledSeverity(severity diagnostic.Severity, text string, palette ui.Palette) string {
	switch severity {
	case diagnostic.SeverityError:
		return palette.Error(text)
	case diagnostic.SeverityNote:
		return palette.Note(text)
	default:
		return palette.Warning(text)
	}
}

func summary(
	diagnostics []diagnostic.Diagnostic,
	counts map[diagnostic.Severity]int,
	palette ui.Palette,
) string {
	label := "issues"
	if len(diagnostics) == 1 {
		label = "issue"
	}
	prefix := fmt.Sprintf("found %d %s", len(diagnostics), label)
	if len(diagnostics) == 0 {
		return palette.Success(prefix)
	}
	parts := make([]string, 0, 3)
	for _, severity := range[]diagnostic.Severity{
		diagnostic.SeverityError,
		diagnostic.SeverityWarning,
		diagnostic.SeverityNote,
	} {
		if count := counts[severity]; count != 0 {
			part := fmt.Sprintf("%d %s", count, plural(string(severity), count))
			parts = append(parts, styledSeverity(severity, part, palette))
		}
	}
	return palette.White(prefix + ": ") + strings.Join(parts, palette.White(", "))
}

func plural(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}
